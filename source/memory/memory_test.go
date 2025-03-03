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

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"

	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
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

func TestDetectHugePages(t *testing.T) {

	Convey("With configured 1Gi huge pages size", t, func() {
		hostpath.SysfsDir = "testdata/hugepages"

		expectedHugePages := map[string]string{
			"enabled":       "true",
			"hugepages-1Gi": "2",
			"hugepages-2Mi": "0",
		}
		hugePages, err := detectHugePages()
		assert.Nil(t, err)
		assert.Equal(t, hugePages, expectedHugePages)
	})

	Convey("With invalid directory structure", t, func() {
		hostpath.SysfsDir = "invalid-dir"

		expectedHugePages := map[string]string{
			"enabled": "false",
		}
		hugePages, err := detectHugePages()
		assert.Nil(t, err)
		assert.Equal(t, hugePages, expectedHugePages)
	})

}

func TestGetHugePagesTotalCount(t *testing.T) {

	Convey("With configured total huge pages", t, func() {
		totalPages, err := getHugePagesTotalCount("testdata/hugepages/kernel/mm/hugepages", "hugepages-1048576kB")
		assert.Equal(t, "2", totalPages)
		assert.Nil(t, err)
	})

	Convey("With not configured total huge pages", t, func() {
		totalPages, err := getHugePagesTotalCount("testdata/hugepages/kernel/mm/hugepages", "hugepages-2048kB")
		assert.Equal(t, "0", totalPages)
		assert.Nil(t, err)
	})

	Convey("With invalid huge page directory", t, func() {
		totalPages, err := getHugePagesTotalCount("testdata/hugepages/kernel/mm/hugepages", "hugepages-invalid")
		assert.Equal(t, "", totalPages)
		assert.NotNil(t, err)
	})

}
