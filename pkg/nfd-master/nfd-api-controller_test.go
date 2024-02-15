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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/api/core/v1"
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

func newTestNamespace(name string) *corev1.Namespace{
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"name": name,
			},
		},
	}
}

func TestIsNamespaceAllowed(t *testing.T) {
	fakeCli := fakeclient.NewSimpleClientset(newTestNamespace("fake"))
	c := &nfdController{
		k8sClient: fakeCli,
	}

	testcases := []struct {
		name                         string
		objectNamespace              string
		nodeFeatureNamespaceSelector *metav1.LabelSelector
		expectedResult               bool
	}{
		{
			name:                         "namespace not allowed",
			objectNamespace:              "random",
			nodeFeatureNamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "name",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"fake"},
					},
				},
			},
			expectedResult:               false,
		},
		{
			name:                         "namespace is allowed",
			objectNamespace:              "fake",
			nodeFeatureNamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"name": "fake"},
			},
			expectedResult:               false,
		},
	}

	for _, tc := range testcases {
		c.nodeFeatureNamespaceSelector = tc.nodeFeatureNamespaceSelector
		res := c.isNamespaceSelected(tc.name)
		assert.Equal(t, res, tc.expectedResult)
	}
}
