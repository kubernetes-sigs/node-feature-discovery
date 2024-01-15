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
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
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

func TestIsNamespaceAllowed(t *testing.T) {
	c := &nfdController{}

	testcases := []struct {
		name              string
		objectNamespace   string
		allowedNamespaces []string
		expectedResult    bool
	}{
		{
			name:              "namespace not allowed",
			objectNamespace:   "ns3",
			allowedNamespaces: []string{"ns1", "ns2"},
			expectedResult:    false,
		},
		{
			name:              "namespace is allowed",
			objectNamespace:   "ns1",
			allowedNamespaces: []string{"ns2", "ns1"},
			expectedResult:    false,
		},
	}

	for _, tc := range testcases {
		c.allowedNamespaces = tc.allowedNamespaces
		res := c.isNamespaceAllowed(tc.name)
		assert.Equal(t, res, tc.expectedResult)
	}
}
