/*
Copyright 2021 The Kubernetes Authors.

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

package memory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemorySource(t *testing.T) {
	assert.Equal(t, src.Name(), Name)

	// Check that GetLabels works with empty features
	src.features = nil
	l, err := src.GetLabels()

	assert.Nil(t, err, err)
	assert.Empty(t, l)
}

func TestGetNumberofLinesFromFile(t *testing.T) {
	type testCase struct {
		path          string
		expectedLines int
		expectErr     bool
	}
	tc := []testCase{
		{
			path:          "testdata/swap",
			expectedLines: 2,
		},
		{
			path:          "testdata/noswap",
			expectedLines: 1,
		},
		{
			path:      "file_not_exist",
			expectErr: true,
		},
	}
	for _, tc := range tc {
		actual, err := getNumberOfNonEmptyLinesFromFile(tc.path)
		if tc.expectErr {
			assert.NotNil(t, err, "should get an error")
		}
		assert.Equal(t, tc.expectedLines, actual, "lines should match")
	}
}
