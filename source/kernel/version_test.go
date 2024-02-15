/*
Copyright 2024 The Kubernetes Authors.

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

package kernel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		raw      string
		expected map[string]string
	}{
		{
			raw: "6.2.3",
			expected: map[string]string{
				"full":     "6.2.3",
				"major":    "6",
				"minor":    "2",
				"revision": "3",
			},
		},
		{
			raw: "6.0.0-beta",
			expected: map[string]string{
				"full":     "6.0.0-beta",
				"major":    "6",
				"minor":    "0",
				"revision": "0",
			},
		},
		{
			raw: "6.8",
			expected: map[string]string{
				"full":     "6.8",
				"major":    "6",
				"minor":    "8",
				"revision": "",
			},
		},
		{
			raw: "6.0.10-100+123.x86_64~",
			expected: map[string]string{
				"full":     "6.0.10-100_123.x86_64",
				"major":    "6",
				"minor":    "0",
				"revision": "10",
			},
		},
	}

	for _, test := range tests {
		actual := parseVersion(test.raw)
		assert.Equal(t, test.expected, actual)
	}
}
