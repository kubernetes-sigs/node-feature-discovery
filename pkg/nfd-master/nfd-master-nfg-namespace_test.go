/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nfdmaster

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	fakenfdclient "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned/fake"
	nfdscheme "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned/scheme"
	nfdinformers "sigs.k8s.io/node-feature-discovery/api/generated/informers/externalversions"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

const (
	nfgTestMasterNamespace = "node-feature-discovery"
	nfgTestOtherNamespace  = "other-namespace"
	nfgTestName            = "test-nfg"
)

// newFakeNfdAPIControllerWithNFG builds a controller whose NodeFeature and
// NodeFeatureGroup listers are backed by (and synced from) the given fake
// client. Unlike newFakeNfdAPIController it also wires the NodeFeatureGroup
// lister, which nfdAPIUpdateAllNodeFeatureGroups needs.
func newFakeNfdAPIControllerWithNFG(client *fakenfdclient.Clientset) *nfdController {
	c := &nfdController{
		stopChan:           make(chan struct{}),
		updateAllNodesChan: make(chan struct{}, 1),
		updateOneNodeChan:  make(chan string),
	}

	informerFactory := nfdinformers.NewSharedInformerFactory(client, 1*time.Hour)

	featureInformer := informerFactory.Nfd().V1alpha1().NodeFeatures()
	c.featureLister = featureInformer.Lister()

	nodeFeatureGroupInformer := informerFactory.Nfd().V1alpha1().NodeFeatureGroups()
	c.featureGroupLister = nodeFeatureGroupInformer.Lister()

	informerFactory.Start(c.stopChan)
	cache.WaitForCacheSync(c.stopChan,
		featureInformer.Informer().HasSynced,
		nodeFeatureGroupInformer.Informer().HasSynced,
	)

	utilruntime.Must(nfdv1alpha1.AddToScheme(nfdscheme.Scheme))

	return c
}

// newNfgTestNodeFeature returns a NodeFeature for testNodeName carrying the
// system.name/nodename attribute that nfdAPIUpdateNodeFeatureGroup reads when
// building the matched-node set.
func newNfgTestNodeFeature() *nfdv1alpha1.NodeFeature {
	return &nfdv1alpha1.NodeFeature{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNodeName + "-features",
			Namespace: nfgTestMasterNamespace,
			Labels: map[string]string{
				nfdv1alpha1.NodeFeatureObjNodeNameLabel: testNodeName,
			},
		},
		Spec: nfdv1alpha1.NodeFeatureSpec{
			Features: nfdv1alpha1.Features{
				Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{
					"system.name": {
						Elements: map[string]string{"nodename": testNodeName},
					},
				},
			},
		},
	}
}

// newNfgMatchAll returns a NodeFeatureGroup in the given namespace with a
// single rule that has no matchers, which matches every node (see
// ExecuteGroupRule: IsMatch is true when both MatchAny and MatchFeatures are
// empty).
func newNfgMatchAll(namespace string) *nfdv1alpha1.NodeFeatureGroup {
	return &nfdv1alpha1.NodeFeatureGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nfgTestName,
			Namespace: namespace,
		},
		Spec: nfdv1alpha1.NodeFeatureGroupSpec{
			Rules: []nfdv1alpha1.GroupRule{
				{Name: "match-all"},
			},
		},
	}
}

// drainNfgQueue processes every item currently in the NodeFeatureGroup queue.
// The updater pool is started with parallelism 0 so no consumer goroutines
// run; this drains the queue synchronously and deterministically.
func drainNfgQueue(t *testing.T, u *updaterPool, cli *fakenfdclient.Clientset) {
	t.Helper()
	for i := 0; i < 100 && u.nfgQueue.Len() > 0; i++ {
		u.processNodeFeatureGroupUpdateRequest(cli)
	}
	require.Zero(t, u.nfgQueue.Len(), "NodeFeatureGroup queue was not drained")
}

// TestNodeFeatureGroupNamespaceScoping exercises the full enqueue -> dequeue ->
// status-write path for NodeFeatureGroups that live in different namespaces.
//
// Regression for #2558. On master this fails because:
//   - the enqueue site keys the queue by bare Name, so two same-name NFGs in
//     different namespaces collide into a single queue entry, and
//   - the consumer resolves the NFG only in nfd-master's own namespace, so an
//     NFG in any other namespace is NotFound and silently skipped, and
//   - the status write is pinned to nfd-master's namespace.
//
// The net effect is that the NFG in nfgTestOtherNamespace never gets its status
// populated. After the fix both NFGs are processed independently in their own
// namespaces.
func TestNodeFeatureGroupNamespaceScoping(t *testing.T) {
	node := newTestNode()
	k8sCli := fakeclient.NewClientset(node)

	nodeFeature := newNfgTestNodeFeature()
	nfgInMaster := newNfgMatchAll(nfgTestMasterNamespace)
	nfgInOther := newNfgMatchAll(nfgTestOtherNamespace)

	//nolint:staticcheck
	nfdCli := fakenfdclient.NewSimpleClientset(nodeFeature, nfgInMaster, nfgInOther)

	fakeMaster := newFakeMaster(WithKubernetesClient(k8sCli), withNFDClient(nfdCli))
	fakeMaster.namespace = nfgTestMasterNamespace
	fakeMaster.nfdController = newFakeNfdAPIControllerWithNFG(nfdCli)

	updaterPool := newUpdaterPool(fakeMaster)
	fakeMaster.updaterPool = updaterPool
	updaterPool.start(0) // initialize the queues without spawning consumer goroutines
	defer updaterPool.stop()

	require.NoError(t, fakeMaster.nfdAPIUpdateAllNodeFeatureGroups())

	drainNfgQueue(t, updaterPool, nfdCli)

	testCases := []struct {
		name      string
		namespace string
	}{
		{name: "NFG in nfd-master namespace", namespace: nfgTestMasterNamespace},
		{name: "NFG in a different namespace", namespace: nfgTestOtherNamespace},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := nfdCli.NfdV1alpha1().NodeFeatureGroups(tc.namespace).Get(
				context.TODO(), nfgTestName, metav1.GetOptions{})
			require.NoError(t, err)

			nodeNames := make([]string, 0, len(got.Status.Nodes))
			for _, n := range got.Status.Nodes {
				nodeNames = append(nodeNames, n.Name)
			}
			assert.ElementsMatch(t, []string{testNodeName}, nodeNames,
				"NodeFeatureGroup %q in namespace %q should have its own status populated",
				nfgTestName, tc.namespace)
		})
	}
}

// TestNfdAPIUpdateNodeFeatureGroupTargetsObjectNamespace asserts that the
// status write targets the NodeFeatureGroup's own namespace rather than
// nfd-master's namespace.
//
// Regression for #2558 (the UpdateStatus namespace pinning). The object lives
// in nfgTestOtherNamespace while nfd-master's namespace is
// nfgTestMasterNamespace. On master the write is issued against nfd-master's
// namespace, which the fake client rejects because the request namespace does
// not match the object's own namespace; after the fix the write targets the
// object's namespace and the recorded UpdateStatus action confirms it.
func TestNfdAPIUpdateNodeFeatureGroupTargetsObjectNamespace(t *testing.T) {
	node := newTestNode()
	k8sCli := fakeclient.NewClientset(node)

	nodeFeature := newNfgTestNodeFeature()
	nfg := newNfgMatchAll(nfgTestOtherNamespace)

	//nolint:staticcheck
	nfdCli := fakenfdclient.NewSimpleClientset(nodeFeature, nfg)

	fakeMaster := newFakeMaster(WithKubernetesClient(k8sCli), withNFDClient(nfdCli))
	fakeMaster.namespace = nfgTestMasterNamespace
	fakeMaster.nfdController = newFakeNfdAPIControllerWithNFG(nfdCli)

	require.NotEqual(t, nfg.Namespace, fakeMaster.namespace,
		"test precondition: object namespace must differ from nfd-master namespace")

	nfdCli.ClearActions()
	require.NoError(t, fakeMaster.nfdAPIUpdateNodeFeatureGroup(nfdCli, nfg))

	var updateNamespaces []string
	for _, action := range nfdCli.Actions() {
		if action.GetVerb() == "update" &&
			action.GetResource().Resource == "nodefeaturegroups" &&
			action.GetSubresource() == "status" {
			updateNamespaces = append(updateNamespaces, action.GetNamespace())
		}
	}

	require.Len(t, updateNamespaces, 1, "expected exactly one NodeFeatureGroup status update")
	assert.Equal(t, nfgTestOtherNamespace, updateNamespaces[0],
		"UpdateStatus must target the NodeFeatureGroup's own namespace, not nfd-master's namespace")
}
