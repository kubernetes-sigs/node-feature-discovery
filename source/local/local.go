/*
Copyright 2018-2021 The Kubernetes Authors.

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
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/fsnotify/fsnotify"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Name of this feature source
const Name = "local"

// LabelFeature of this feature source
const LabelFeature = "label"

// RawFeature of this feature source
const RawFeature = "feature"

const (
	// ExpiryTimeKey is the key of this feature source indicating
	// when features should be removed.
	DirectiveExpiryTime = "expiry-time"

	// NoLabel indicates whether the feature should be included
	// in exposed labels or not.
	DirectiveNoLabel = "no-label"

	// NoFeature indicates whether the feature should be included
	// in exposed raw features or not.
	DirectiveNoFeature = "no-feature"
)

// DirectivePrefix defines the prefix of directives that should be parsed
const DirectivePrefix = "# +"

// MaxFeatureFileSize defines the maximum size of a feature file size
const MaxFeatureFileSize = 65536

// Config
var (
	featureFilesDir = "/etc/kubernetes/node-feature-discovery/features.d/"
)

// localSource implements the FeatureSource, LabelSource, EventSource interfaces.
type localSource struct {
	features   *nfdv1alpha1.Features
	config     *Config
	cancelFunc context.CancelFunc // cancels the active notifier goroutine
	done       chan struct{}      // closed when notifier goroutine exits
	mu         sync.Mutex         // serializes SetNotifyChannel and protects fields
}

type Config struct {
}

// parsingOpts contains options used for directives parsing
type parsingOpts struct {
	ExpiryTime  time.Time
	SkipLabel   bool
	SkipFeature bool
}

// Singleton source instance
var (
	src                           = localSource{}
	_   source.FeatureSource      = &src
	_   source.LabelSource        = &src
	_   source.ConfigurableSource = &src
	_   source.EventSource        = &src
)

// Name method of the LabelSource interface
func (s *localSource) Name() string { return Name }

// NewConfig method of the LabelSource interface
func (s *localSource) NewConfig() source.Config { return &Config{} }

// GetConfig method of the LabelSource interface
func (s *localSource) GetConfig() source.Config { return s.config }

// SetConfig method of the LabelSource interface
func (s *localSource) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
	default:
		panic(fmt.Sprintf("invalid config type: %T", conf))
	}
}

// Priority method of the LabelSource interface
func (s *localSource) Priority() int { return 20 }

// GetLabels method of the LabelSource interface
func (s *localSource) GetLabels() (source.FeatureLabels, error) {
	labels := make(source.FeatureLabels)
	features := s.GetFeatures()

	for k, v := range features.Attributes[LabelFeature].Elements {
		labels[k] = v
	}
	return labels, nil
}

// Discover method of the FeatureSource interface
func (s *localSource) Discover() error {
	s.features = nfdv1alpha1.NewFeatures()

	featuresFromFiles, labelsFromFiles, err := getFeaturesFromFiles()
	if err != nil {
		klog.ErrorS(err, "failed to read feature files")
	}

	s.features.Attributes[LabelFeature] = nfdv1alpha1.NewAttributeFeatures(labelsFromFiles)
	s.features.Attributes[RawFeature] = nfdv1alpha1.NewAttributeFeatures(featuresFromFiles)

	klog.V(3).InfoS("discovered features", "featureSource", s.Name(), "features", utils.DelayedDumper(s.features))

	return nil
}

// GetFeatures method of the FeatureSource Interface
func (s *localSource) GetFeatures() *nfdv1alpha1.Features {
	if s.features == nil {
		s.features = nfdv1alpha1.NewFeatures()
	}
	return s.features
}

func parseDirectives(line string, opts *parsingOpts) error {
	if !strings.HasPrefix(line, DirectivePrefix) {
		return nil
	}

	directive := line[len(DirectivePrefix):]
	split := strings.SplitN(directive, "=", 2)
	key := split[0]

	switch key {
	case DirectiveExpiryTime:
		if len(split) == 1 {
			return fmt.Errorf("invalid directive format in %q, should be '# +expiry-time=value'", line)
		}
		value := split[1]
		expiryDate, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("failed to parse expiry-date directive: %w", err)
		}
		opts.ExpiryTime = expiryDate
	case DirectiveNoFeature:
		opts.SkipFeature = true
	case DirectiveNoLabel:
		opts.SkipLabel = true
	default:
		return fmt.Errorf("unknown feature file directive %q", key)
	}

	return nil
}

func parseFeatureFile(lines [][]byte, fileName string) (map[string]string, map[string]string) {
	features := make(map[string]string)
	labels := make(map[string]string)

	now := time.Now()
	parsingOpts := &parsingOpts{
		ExpiryTime:  now,
		SkipLabel:   false,
		SkipFeature: false,
	}

	for _, l := range lines {
		line := strings.TrimSpace(string(l))
		if len(line) > 0 {
			if strings.HasPrefix(line, "#") {
				// Parse directives
				err := parseDirectives(line, parsingOpts)
				if err != nil {
					klog.ErrorS(err, "error while parsing directives", "fileName", fileName)
				}

				continue
			}

			// handle expiration
			if parsingOpts.ExpiryTime.Before(now) {
				continue
			}

			lineSplit := strings.SplitN(line, "=", 2)

			key := lineSplit[0]

			if !parsingOpts.SkipFeature {
				updateFeatures(features, lineSplit)
			} else {
				delete(features, key)
			}

			if !parsingOpts.SkipLabel {
				updateFeatures(labels, lineSplit)
			} else {
				delete(labels, key)
			}
			// SkipFeature and SkipLabel only take effect for one feature
			parsingOpts.SkipFeature = false
			parsingOpts.SkipLabel = false
		}
	}

	return features, labels
}

func updateFeatures(m map[string]string, lineSplit []string) {
	key := lineSplit[0]
	// Check if it's a boolean value
	if len(lineSplit) == 1 {
		m[key] = "true"

	} else {
		m[key] = lineSplit[1]
	}
}

// Read all files to get features
func getFeaturesFromFiles() (map[string]string, map[string]string, error) {
	features := make(map[string]string)
	labels := make(map[string]string)

	files, err := os.ReadDir(featureFilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			klog.InfoS("features directory does not exist", "path", featureFilesDir)
			return features, labels, nil
		}
		return features, labels, fmt.Errorf("unable to access %v: %w", featureFilesDir, err)
	}

	for _, file := range files {
		fileName := file.Name()
		// ignore hidden feature file
		if strings.HasPrefix(fileName, ".") {
			continue
		}
		lines, err := getFileContent(fileName)
		if err != nil {
			klog.ErrorS(err, "failed to read file", "fileName", fileName)
			continue
		}

		// Append features
		fileFeatures, fileLabels := parseFeatureFile(lines, fileName)

		klog.V(4).InfoS("feature file read", "fileName", fileName, "features", utils.DelayedDumper(fileFeatures))
		for k, v := range fileFeatures {
			if old, ok := features[k]; ok {
				klog.InfoS("overriding label value from another feature file", "featureKey", k, "oldValue", old, "newValue", v, "fileName", fileName)
			}
			features[k] = v
		}

		for k, v := range fileLabels {
			if old, ok := labels[k]; ok {
				klog.InfoS("overriding label value from another feature file", "labelKey", k, "oldValue", old, "newValue", v, "fileName", fileName)
			}
			labels[k] = v
		}
	}

	return features, labels, nil
}

// Read one file
func getFileContent(fileName string) ([][]byte, error) {
	var lines [][]byte

	path := filepath.Join(featureFilesDir, fileName)
	filestat, err := os.Stat(path)
	if err != nil {
		klog.ErrorS(err, "failed to get filestat, skipping features file", "path", path)
		return lines, err
	}

	if filestat.Mode().IsRegular() {
		if filestat.Size() > MaxFeatureFileSize {
			return lines, fmt.Errorf("file size limit exceeded: %d bytes > %d bytes", filestat.Size(), MaxFeatureFileSize)
		}

		fileContent, err := os.ReadFile(path)

		// Do not return any lines if an error occurred
		if err != nil {
			return lines, err
		}
		lines = bytes.Split(fileContent, []byte("\n"))
	}

	return lines, nil
}

func (s *localSource) runNotifier(ctx context.Context, ch chan *source.FeatureSource, watcher *fsnotify.Watcher, done chan struct{}) {
	defer close(done)
	rateLimit := time.NewTicker(time.Second)
	defer rateLimit.Stop()
	defer func() {
		// Each goroutine is responsible for closing its own watcher
		if err := watcher.Close(); err != nil {
			klog.ErrorS(err, "failed to close fsnotify watcher")
		}
	}()
	limit := false
	for {
		select {
		case event := <-watcher.Events:
			opAny := fsnotify.Create | fsnotify.Write | fsnotify.Remove | fsnotify.Rename | fsnotify.Chmod
			if event.Op&opAny != 0 {
				klog.V(2).InfoS("fsnotify event", "eventName", event.Name, "eventOp", event.Op)
				if !limit {
					fs := source.FeatureSource(s)
					ch <- &fs
					limit = true
				}
			}
		case err := <-watcher.Errors:
			klog.ErrorS(err, "failed to watch features.d changes")
		case <-rateLimit.C:
			limit = false
		case <-ctx.Done():
			return
		}
	}
}

// stopNotifier cancels the active notifier goroutine.
// The goroutine is responsible for closing its own watcher.
// Must be called with s.mu held.
func (s *localSource) stopNotifier() {
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil
	}
}

// createWatcher creates a new fsnotify watcher for the feature files directory.
func createWatcher() (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(featureFilesDir); err != nil {
		if errClose := watcher.Close(); errClose != nil {
			klog.ErrorS(errClose, "failed to close fsnotify watcher")
		}
		return nil, fmt.Errorf("unable to access %v: %w", featureFilesDir, err)
	}

	return watcher, nil
}

// SetNotifyChannel method of the EventSource Interface
func (s *localSource) SetNotifyChannel(ctx context.Context, ch chan *source.FeatureSource) error {
	info, err := os.Stat(featureFilesDir)
	if err != nil {
		return err
	}

	if info.IsDir() {
		s.mu.Lock()
		defer s.mu.Unlock()

		// Create watcher under lock to prevent concurrent calls from
		// creating multiple watchers where some leak due to race conditions.
		watcher, err := createWatcher()
		if err != nil {
			return err
		}

		// Stop any existing notifier
		s.stopNotifier()
		prevDone := s.done

		// Wait for the previous goroutine to fully exit before starting a new one.
		// Safe to wait under lock since runNotifier doesn't acquire mu.
		if prevDone != nil {
			<-prevDone
		}

		// Create a cancellable context for the notifier goroutine
		notifyCtx, cancel := context.WithCancel(ctx)
		s.cancelFunc = cancel
		s.done = make(chan struct{})

		go s.runNotifier(notifyCtx, ch, watcher, s.done)
	}

	return nil
}

func init() {
	source.Register(&src)
}
