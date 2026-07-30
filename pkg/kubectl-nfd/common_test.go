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

package kubectlnfd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

func TestParseRuleFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		missing  bool
		expected interface{}
	}{
		{
			name: "NodeFeatureRule",
			content: `apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: test-rule
spec:
  rules:
  - name: test rule
    labels:
      test-label: "true"
`,
			expected: &nfdv1alpha1.NodeFeatureRule{},
		},
		{
			name: "NodeFeatureGroup",
			content: `apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureGroup
metadata:
  name: test-group
spec:
  rules:
  - name: test rule
`,
			expected: &nfdv1alpha1.NodeFeatureGroup{},
		},
		{
			name: "unsupported kind",
			content: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-configmap
`,
			expected: []error{},
		},
		{
			name:     "malformed YAML",
			content:  "kind: [this is not valid yaml",
			expected: []error{},
		},
		{
			name:     "missing file",
			missing:  true,
			expected: []error{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var path string
			if test.missing {
				path = filepath.Join(t.TempDir(), "does-not-exist.yaml")
			} else {
				path = filepath.Join(t.TempDir(), "rule.yaml")
				assert.NoError(t, os.WriteFile(path, []byte(test.content), 0644))
			}

			obj := parseRuleFile(path)

			switch test.expected.(type) {
			case *nfdv1alpha1.NodeFeatureRule:
				assert.IsType(t, &nfdv1alpha1.NodeFeatureRule{}, obj)
			case *nfdv1alpha1.NodeFeatureGroup:
				assert.IsType(t, &nfdv1alpha1.NodeFeatureGroup{}, obj)
			case []error:
				assert.IsType(t, []error{}, obj)
				errs, ok := obj.([]error)
				assert.True(t, ok)
				assert.NotEmpty(t, errs)
			}
		})
	}
}
