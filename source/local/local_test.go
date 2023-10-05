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

package local

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLocalSource(t *testing.T) {
	assert.Equal(t, src.Name(), Name)

	// Check that GetLabels works with empty features
	src.features = nil
	l, err := src.GetLabels()

	assert.Nil(t, err, err)
	assert.Empty(t, l)

}

func TestGetExpirationDate(t *testing.T) {
	expectedFeaturesLen := 7
	expectedLabelsLen := 8

	pwd, _ := os.Getwd()
	featureFilesDir = filepath.Join(pwd, "testdata/features.d")
	features, labels, err := getFeaturesFromFiles()

	assert.NoError(t, err)
	assert.Equal(t, expectedFeaturesLen, len(features))
	assert.Equal(t, expectedLabelsLen, len(labels))
}

func TestParseDirectives(t *testing.T) {
	testCases := []struct {
		name      string
		directive string
		wantErr   bool
	}{
		{
			name:      "valid directive",
			directive: "# +expiry-time=2080-07-28T11:22:33Z",
			wantErr:   false,
		},
		{
			name:      "invalid directive",
			directive: "# +random-key=random-value",
			wantErr:   true,
		},
		{
			name:      "invalid directive format",
			directive: "# + Something",
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsingOpts := parsingOpts{
				ExpiryTime: time.Now(),
			}
			err := parseDirectives(tc.directive, &parsingOpts)
			assert.Equal(t, err != nil, tc.wantErr)
		})
	}
}
