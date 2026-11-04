/*
Copyright 2023 The Kubernetes Authors.

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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	clientfeatures "k8s.io/client-go/features"
	clientfeaturestesting "k8s.io/client-go/features/testing"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
	clienttesting "k8s.io/client-go/testing"

	nfdclientset "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned"
	nfdinformers "sigs.k8s.io/node-feature-discovery/api/generated/informers/externalversions"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

func TestNodeFeatureInformerPagination(t *testing.T) {
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)

	testCases := []struct {
		name       string
		listSize   int64
		expirePage bool
		relist     bool
		wantLimits []int64
	}{
		{name: "paginated list", listSize: 2, wantLimits: []int64{2, 2}},
		{name: "expired continuation", listSize: 2, expirePage: true, wantLimits: []int64{2, 2, 2, 2, 0}},
		{name: "relist after watch failure", listSize: 2, relist: true, wantLimits: []int64{2, 2, 2}},
		{name: "pagination disabled", listSize: 0, wantLimits: []int64{0}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			features := []nfdv1alpha1.NodeFeature{
				{ObjectMeta: metav1.ObjectMeta{Name: "feature-a", Namespace: "default", Labels: map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: "node-a"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "feature-b", Namespace: "default", Labels: map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: "node-b"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "feature-c", Namespace: "default", Labels: map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: "node-c"}}},
			}
			var mu sync.Mutex
			var limits []int64
			var resourceVersions []string
			watchCount := 0
			relistReady := make(chan struct{})
			t.Cleanup(func() {
				mu.Lock()
				defer mu.Unlock()
				t.Logf("LIST request limits: %v", limits)
			})
			httpClient := restfake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
				response := &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}
				var body bytes.Buffer
				response.Body = io.NopCloser(&body)
				if r.URL.Query().Get("watch") == "true" {
					mu.Lock()
					watchCount++
					currentWatch := watchCount
					mu.Unlock()
					if tc.relist && currentWatch == 2 {
						response.StatusCode = http.StatusInternalServerError
						err := apierrors.NewInternalError(assert.AnError)
						err.ErrStatus.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}
						assert.NoError(t, json.NewEncoder(&body).Encode(err.ErrStatus))
						return response, nil
					}
					reader, writer := io.Pipe()
					response.Body = reader
					go func() {
						defer func() { assert.NoError(t, writer.Close()) }()
						if tc.relist && currentWatch == 1 {
							// A bookmark makes this a successful watch even when it closes quickly.
							assert.NoError(t, json.NewEncoder(writer).Encode(map[string]any{
								"type": "BOOKMARK", "object": map[string]any{
									"apiVersion": nfdv1alpha1.SchemeGroupVersion.String(), "kind": "NodeFeature",
									"metadata": map[string]string{"resourceVersion": "1"},
								},
							}))
							select {
							case <-relistReady:
							case <-r.Context().Done():
							}
							return
						}
						<-r.Context().Done()
					}()
					return response, nil
				}

				limit := int64(0)
				if value := r.URL.Query().Get("limit"); value != "" {
					var err error
					limit, err = strconv.ParseInt(value, 10, 64)
					if !assert.NoError(t, err) {
						response.StatusCode = http.StatusBadRequest
						return response, nil
					}
				}
				mu.Lock()
				limits = append(limits, limit)
				resourceVersions = append(resourceVersions, r.URL.Query().Get("resourceVersion"))
				initialList := tc.relist && watchCount == 0
				mu.Unlock()

				start := 0
				if r.URL.Query().Get("continue") != "" {
					if tc.expirePage {
						response.StatusCode = http.StatusGone
						err := apierrors.NewResourceExpired("continuation expired")
						err.ErrStatus.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}
						assert.NoError(t, json.NewEncoder(&body).Encode(err.ErrStatus))
						return response, nil
					}
					start = 2
				}
				end := len(features)
				if initialList {
					end = 1
				}
				if limit > 0 && int64(end-start) > limit {
					end = start + int(limit)
				}
				result := nfdv1alpha1.NodeFeatureList{
					TypeMeta: metav1.TypeMeta{APIVersion: nfdv1alpha1.SchemeGroupVersion.String(), Kind: "NodeFeatureList"},
					ListMeta: metav1.ListMeta{ResourceVersion: "1"},
					Items:    features[start:end],
				}
				if !initialList && end < len(features) {
					result.Continue = "next"
				}
				assert.NoError(t, json.NewEncoder(&body).Encode(result))
				return response, nil
			})

			client, err := nfdclientset.NewForConfigAndClient(&rest.Config{Host: "http://nfd.test"}, httpClient)
			require.NoError(t, err)
			factory := nfdinformers.NewSharedInformerFactory(client, 0)
			controller := &nfdController{
				updateOneNodeChan:       make(chan string, len(features)),
				disableNodeFeatureGroup: true,
			}
			require.NoError(t, controller.addNodeFeatureInformer(factory, tc.listSize))
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			t.Cleanup(func() {
				cancel()
				factory.Shutdown()
			})
			factory.Start(ctx.Done())
			for resource, synced := range factory.WaitForCacheSync(ctx.Done()) {
				require.True(t, synced, "informer failed to sync: %v", resource)
			}

			if tc.relist {
				cached, err := controller.featureLister.List(labels.Everything())
				require.NoError(t, err)
				require.Len(t, cached, 1, "startup must fit in one page so the reflector does not paginate relists")
				close(relistReady)
				require.Eventually(t, func() bool {
					cached, err := controller.featureLister.List(labels.Everything())
					return err == nil && len(cached) == len(features)
				}, 5*time.Second, 10*time.Millisecond, "relist did not load all NodeFeatures")
			}

			cached, err := controller.featureLister.List(labels.Everything())
			require.NoError(t, err)
			names := make([]string, 0, len(cached))
			for _, feature := range cached {
				names = append(names, feature.Name)
			}
			assert.ElementsMatch(t, []string{"feature-a", "feature-b", "feature-c"}, names)
			mu.Lock()
			assert.Equal(t, tc.wantLimits, limits)
			if tc.relist {
				assert.Equal(t, []string{"", "1", ""}, resourceVersions)
			}
			mu.Unlock()
		})
	}
}

func TestGetNodeNameForObj(t *testing.T) {
	// Test missing label
	obj := &nfdv1alpha1.NodeFeature{Name: "foo", Namespace: "bar"}
	_, err := getNodeNameForObj(obj)
	assert.Error(t, err)

	// Test empty label
	obj.SetLabels(map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: ""})
	_, err = getNodeNameForObj(obj)
	assert.Error(t, err)

	// Test proper node name
	obj.SetLabels(map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: "node-1"})
	n, err := getNodeNameForObj(obj)
	assert.Nil(t, err)
	assert.Equal(t, n, "node-1")
}

func TestControllerStop(t *testing.T) {
	// stop() must not panic when no NodeFeature namespace selector was
	// configured and namespaceLister is therefore nil (the default).
	c := &nfdController{stopChan: make(chan struct{})}
	assert.NotPanics(t, c.stop)
}

func newTestNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		Name: name,
		Labels: map[string]string{
			"name": name,
		},
	}
}

func TestIsNamespaceSelected(t *testing.T) {
	fakeCli := fakeclient.NewClientset(newTestNamespace("fake"))
	fakeCli.PrependWatchReactor("*", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := fakeCli.Tracker().Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})

	c := &nfdController{}

	testcases := []struct {
		name                         string
		objectNamespace              string
		nodeFeatureNamespaceSelector *metav1.LabelSelector
		expectedResult               bool
	}{
		{
			name:            "namespace not selected",
			objectNamespace: "random",
			nodeFeatureNamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "name",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"fake"},
					},
				},
			},
			expectedResult: false,
		},
		{
			name:            "namespace is selected",
			objectNamespace: "fake",
			nodeFeatureNamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"name": "fake"},
			},
			expectedResult: true,
		},
	}

	for _, tc := range testcases {
		labelMap, _ := metav1.LabelSelectorAsSelector(tc.nodeFeatureNamespaceSelector)
		lister, err := newNamespaceLister(fakeCli, labelMap)
		assert.Nil(t, err)
		c.namespaceLister = lister
		res := c.isNamespaceSelected(tc.objectNamespace)
		assert.Equal(t, res, tc.expectedResult)
	}
}

func TestSpecChanged(t *testing.T) {
	nfSpec := func(o *nfdv1alpha1.NodeFeature) any { return o.Spec }
	nfrSpec := func(o *nfdv1alpha1.NodeFeatureRule) any { return o.Spec }
	nfgSpec := func(o *nfdv1alpha1.NodeFeatureGroup) any { return o.Spec }

	// NodeFeature: Spec{Features, Labels}, no status subresource.
	nf := &nfdv1alpha1.NodeFeature{
		Name: "n", ResourceVersion: "1",
		Spec: nfdv1alpha1.NodeFeatureSpec{
			Features: nfdv1alpha1.Features{
				Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{
					"cpu.model": {Elements: map[string]string{"family": "6"}},
				},
			},
			Labels: map[string]string{"feature.node.kubernetes.io/foo": "bar"},
		},
	}
	nfMetaOnly := nf.DeepCopy()
	nfMetaOnly.ResourceVersion = "2"
	nfMetaOnly.Annotations = map[string]string{"nfd.node.kubernetes.io/worker.version": "v0.18"}
	nfMetaOnly.Labels = map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: "node-1"}

	nfFeatureChanged := nf.DeepCopy()
	nfFeatureChanged.Spec.Features.Attributes["cpu.model"] = nfdv1alpha1.AttributeFeatureSet{Elements: map[string]string{"family": "7"}}

	nfLabelsChanged := nf.DeepCopy()
	nfLabelsChanged.Spec.Labels["feature.node.kubernetes.io/foo"] = "baz"

	// NodeFeatureRule: Spec{Rules}, no status. UpdateFunc triggers a full reconcile.
	nfr := &nfdv1alpha1.NodeFeatureRule{
		Name: "r", ResourceVersion: "1",
		Spec: nfdv1alpha1.NodeFeatureRuleSpec{Rules: []nfdv1alpha1.Rule{{Name: "rule-1"}}},
	}
	nfrMetaOnly := nfr.DeepCopy()
	nfrMetaOnly.ResourceVersion = "2"
	nfrMetaOnly.Annotations = map[string]string{"x": "y"}

	nfrSpecChanged := nfr.DeepCopy()
	nfrSpecChanged.Spec.Rules[0].Name = "rule-2"

	// NodeFeatureGroup: Spec{Rules} AND Status; master writes its status, so a
	// status-only update must NOT reconcile (else it feeds back on itself).
	nfg := &nfdv1alpha1.NodeFeatureGroup{
		Name: "g", ResourceVersion: "1",
		Spec: nfdv1alpha1.NodeFeatureGroupSpec{Rules: []nfdv1alpha1.GroupRule{{Name: "grule-1"}}},
	}
	nfgStatusOnly := nfg.DeepCopy()
	nfgStatusOnly.ResourceVersion = "2"
	nfgStatusOnly.Status = nfdv1alpha1.NodeFeatureGroupStatus{Nodes: []nfdv1alpha1.FeatureGroupNode{{Name: "node-1"}}}

	nfgSpecChanged := nfg.DeepCopy()
	nfgSpecChanged.Spec.Rules[0].Name = "grule-2"

	testcases := []struct {
		name string
		got  bool
		want bool
	}{
		{"NodeFeature resync no-op", specChanged(nf, nf.DeepCopy(), nfSpec), false},
		{"NodeFeature metadata-only change", specChanged(nf, nfMetaOnly, nfSpec), false},
		{"NodeFeature feature change", specChanged(nf, nfFeatureChanged, nfSpec), true},
		{"NodeFeature spec.Labels change", specChanged(nf, nfLabelsChanged, nfSpec), true},

		{"NodeFeatureRule metadata-only change", specChanged(nfr, nfrMetaOnly, nfrSpec), false},
		{"NodeFeatureRule spec change", specChanged(nfr, nfrSpecChanged, nfrSpec), true},

		{"NodeFeatureGroup status-only change", specChanged(nfg, nfgStatusOnly, nfgSpec), false},
		{"NodeFeatureGroup spec change", specChanged(nfg, nfgSpecChanged, nfgSpec), true},

		// Fail open: never skip a reconcile when the objects can't be compared.
		{"nil old object", specChanged(nil, nf, nfSpec), true},
		{"mismatched types", specChanged(&nfdv1alpha1.NodeFeatureRule{}, nf, nfSpec), true},
	}

	for _, tc := range testcases {
		assert.Equalf(t, tc.want, tc.got, tc.name)
	}
}

func TestNodeFeaturePublishMetadataChanged(t *testing.T) {
	base := &nfdv1alpha1.NodeFeature{
		Name:            "node-1",
		ResourceVersion: "1",
		Annotations: map[string]string{
			nfdv1alpha1.WorkerVersionAnnotation: "v0.19.0",
		},
	}

	resourceVersionOnly := base.DeepCopy()
	resourceVersionOnly.ResourceVersion = "2"

	workerVersionOnly := base.DeepCopy()
	workerVersionOnly.Annotations[nfdv1alpha1.WorkerVersionAnnotation] = "v0.19.1"

	ownerChanged := base.DeepCopy()
	ownerChanged.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "Node", Name: "node-1", UID: "new-node-uid"}}

	workerPodChanged := base.DeepCopy()
	workerPodChanged.Annotations[nfdv1alpha1.WorkerPodUIDAnnotation] = "new-worker-pod-uid"

	testCases := []struct {
		name string
		old  any
		new  any
		want bool
	}{
		{name: "resource version only", old: base, new: resourceVersionOnly, want: false},
		{name: "worker version only", old: base, new: workerVersionOnly, want: false},
		{name: "owner references", old: base, new: ownerChanged, want: true},
		{name: "worker Pod identity", old: base, new: workerPodChanged, want: true},
		{name: "unexpected object", old: &nfdv1alpha1.NodeFeatureRule{}, new: base, want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, nodeFeaturePublishMetadataChanged(tc.old, tc.new))
		})
	}
}

func TestNodeFeatureMetadataUpdateQueuesOnlyTargetNode(t *testing.T) {
	oldNF := &nfdv1alpha1.NodeFeature{
		Name: "node-1",
		Labels: map[string]string{
			nfdv1alpha1.NodeFeatureObjNodeNameLabel: "node-1",
		},
	}
	newNF := oldNF.DeepCopy()
	newNF.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "Node", Name: "node-1", UID: "node-uid"}}

	c := &nfdController{
		stopChan:                       make(chan struct{}),
		updateOneNodeChan:              make(chan string, 1),
		updateAllNodeFeatureGroupsChan: make(chan struct{}, 1),
	}
	c.onNodeFeatureUpdate(oldNF, newNF)

	select {
	case got := <-c.updateOneNodeChan:
		assert.Equal(t, "node-1", got)
	default:
		t.Fatal("target node was not queued")
	}
	select {
	case <-c.updateAllNodeFeatureGroupsChan:
		t.Fatal("metadata-only update unexpectedly queued all NodeFeatureGroups")
	default:
	}
}
