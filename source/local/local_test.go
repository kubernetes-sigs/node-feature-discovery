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
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/node-feature-discovery/source"
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

func TestSetNotifyChannel_WatcherCleanup(t *testing.T) {
	// Create a temp directory to use as features.d
	tmpDir := t.TempDir()
	originalDir := featureFilesDir
	featureFilesDir = tmpDir
	t.Cleanup(func() { featureFilesDir = originalDir })

	// Create a fresh localSource for testing
	testSrc := &localSource{}

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *source.FeatureSource, 1)

	// Start the notifier
	err := testSrc.SetNotifyChannel(ctx, ch)
	assert.NoError(t, err)

	// Verify cancelFunc and done are set
	testSrc.mu.Lock()
	assert.NotNil(t, testSrc.cancelFunc)
	done := testSrc.done
	testSrc.mu.Unlock()

	// Cancel context and wait for goroutine to exit
	cancel()
	<-done

	// After goroutine exits, watcher should be closed (verified by no panic/hang)
}

func TestSetNotifyChannel_Reinitialization(t *testing.T) {
	// Create a temp directory to use as features.d
	tmpDir := t.TempDir()
	originalDir := featureFilesDir
	featureFilesDir = tmpDir
	t.Cleanup(func() { featureFilesDir = originalDir })

	testSrc := &localSource{}
	ch := make(chan *source.FeatureSource, 1)

	// First call to SetNotifyChannel
	ctx1, cancel1 := context.WithCancel(context.Background())
	err := testSrc.SetNotifyChannel(ctx1, ch)
	assert.NoError(t, err)

	// Second call should stop the first notifier and start a new one
	ctx2, cancel2 := context.WithCancel(context.Background())
	err = testSrc.SetNotifyChannel(ctx2, ch)
	assert.NoError(t, err)

	// First context's cancel should have no effect now (already stopped)
	cancel1()

	// Verify the second notifier is still active
	testSrc.mu.Lock()
	assert.NotNil(t, testSrc.cancelFunc)
	done := testSrc.done
	testSrc.mu.Unlock()

	// Cleanup
	cancel2()
	<-done
}

func TestSetNotifyChannel_ConcurrentCalls(t *testing.T) {
	// Create a temp directory to use as features.d
	tmpDir := t.TempDir()
	originalDir := featureFilesDir
	featureFilesDir = tmpDir
	t.Cleanup(func() { featureFilesDir = originalDir })

	testSrc := &localSource{}
	ch := make(chan *source.FeatureSource, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Make multiple concurrent calls to SetNotifyChannel
	var wg sync.WaitGroup
	numCalls := 10
	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = testSrc.SetNotifyChannel(ctx, ch)
		}()
	}

	// Wait for all calls to complete
	wg.Wait()

	// Verify only one notifier is active
	testSrc.mu.Lock()
	assert.NotNil(t, testSrc.cancelFunc)
	done := testSrc.done
	testSrc.mu.Unlock()

	// Cleanup
	cancel()
	<-done
}
