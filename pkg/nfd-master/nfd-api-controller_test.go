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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

func TestGetNodeNameForObj(t *testing.T) {
	// Test missing label
	obj := &nfdv1alpha1.NodeFeature{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"}}
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
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"name": name,
			},
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
		ObjectMeta: metav1.ObjectMeta{Name: "n", ResourceVersion: "1"},
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
		ObjectMeta: metav1.ObjectMeta{Name: "r", ResourceVersion: "1"},
		Spec:       nfdv1alpha1.NodeFeatureRuleSpec{Rules: []nfdv1alpha1.Rule{{Name: "rule-1"}}},
	}
	nfrMetaOnly := nfr.DeepCopy()
	nfrMetaOnly.ResourceVersion = "2"
	nfrMetaOnly.Annotations = map[string]string{"x": "y"}

	nfrSpecChanged := nfr.DeepCopy()
	nfrSpecChanged.Spec.Rules[0].Name = "rule-2"

	// NodeFeatureGroup: Spec{Rules} AND Status; master writes its status, so a
	// status-only update must NOT reconcile (else it feeds back on itself).
	nfg := &nfdv1alpha1.NodeFeatureGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "g", ResourceVersion: "1"},
		Spec:       nfdv1alpha1.NodeFeatureGroupSpec{Rules: []nfdv1alpha1.GroupRule{{Name: "grule-1"}}},
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
