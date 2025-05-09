/*
Copyright 2025 The Kubernetes Authors.

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

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLibcDetection(t *testing.T) {
	tc := []struct {
		path        string
		expected    libcType
		expectedVer string
		expectErr   bool
	}{
		{
			path:        "testdata/libc/glibc",
			expected:    glibcType,
			expectedVer: "2.39",
			expectErr:   false,
		},
		{
			path:        "testdata/libc/musl",
			expected:    muslType,
			expectedVer: "",
			expectErr:   false,
		},
		{
			path:        "testdata/libc/unknown",
			expected:    unknownType,
			expectedVer: "",
			expectErr:   false,
		},
	}

	for _, test := range tc {
		libcType, version, err := detectLibcImplementation(test.path)
		assert.Equal(t, test.expected, libcType, "libc type should match")
		assert.Equal(t, test.expectedVer, version, "libc version should match")
		assert.Equal(t, test.expectErr, err != nil, "error should match")
	}
}
