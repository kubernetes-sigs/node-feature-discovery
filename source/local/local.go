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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Name of this feature source
const Name = "local"

// LabelFeature of this feature source
const LabelFeature = "label"

// Config
var (
	featureFilesDir = "/etc/kubernetes/node-feature-discovery/features.d/"
	hookDir         = "/etc/kubernetes/node-feature-discovery/source.d/"
)

// localSource implements the FeatureSource and LabelSource interfaces.
type localSource struct {
	features *nfdv1alpha1.Features
	config   *Config
}

type Config struct {
	HooksEnabled bool `json:"hooksEnabled,omitempty"`
}

// Singleton source instance
var (
	src                           = localSource{config: newDefaultConfig()}
	_   source.FeatureSource      = &src
	_   source.LabelSource        = &src
	_   source.ConfigurableSource = &src
)

// Name method of the LabelSource interface
func (s *localSource) Name() string { return Name }

// NewConfig method of the LabelSource interface
func (s *localSource) NewConfig() source.Config { return newDefaultConfig() }

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

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *Config {
	return &Config{
		HooksEnabled: true,
	}
}

// Discover method of the FeatureSource interface
func (s *localSource) Discover() error {
	s.features = nfdv1alpha1.NewFeatures()

	featuresFromFiles, err := getFeaturesFromFiles()
	if err != nil {
		klog.ErrorS(err, "failed to read feature files")
	}

	if s.config.HooksEnabled {

		klog.InfoS("starting hooks...")

		featuresFromHooks, err := getFeaturesFromHooks()
		if err != nil {
			klog.ErrorS(err, "failed to run hooks")
		}

		// Merge features from hooks and files
		for k, v := range featuresFromHooks {
			if old, ok := featuresFromFiles[k]; ok {
				klog.InfoS("overriding label value", "labelKey", k, "oldValue", old, "newValue", v)
			}
			featuresFromFiles[k] = v
		}
	}

	s.features.Attributes[LabelFeature] = nfdv1alpha1.NewAttributeFeatures(featuresFromFiles)

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

func parseFeatures(lines [][]byte) map[string]string {
	features := make(map[string]string)

	for _, line := range lines {
		if len(line) > 0 {
			lineSplit := strings.SplitN(string(line), "=", 2)

			key := lineSplit[0]

			// Check if it's a boolean value
			if len(lineSplit) == 1 {
				features[key] = "true"
			} else {
				features[key] = lineSplit[1]
			}
		}
	}

	return features
}

// Run all hooks and get features
func getFeaturesFromHooks() (map[string]string, error) {

	features := make(map[string]string)

	files, err := os.ReadDir(hookDir)
	if err != nil {
		if os.IsNotExist(err) {
			klog.InfoS("hook directory does not exist", "path", hookDir)
			return features, nil
		}
		return features, fmt.Errorf("unable to access %v: %v", hookDir, err)
	}
	if len(files) > 0 {
		klog.InfoS("hooks are DEPRECATED since v0.12.0 and support will be removed in a future release; use feature files instead")
	}

	for _, file := range files {
		fileName := file.Name()
		lines, err := runHook(fileName)
		if err != nil {
			klog.ErrorS(err, "failed to run hook", "fileName", fileName)
			continue
		}

		// Append features
		fileFeatures := parseFeatures(lines)
		klog.V(4).InfoS("hook executed", "fileName", fileName, "features", utils.DelayedDumper(fileFeatures))
		for k, v := range fileFeatures {
			if old, ok := features[k]; ok {
				klog.InfoS("overriding label value from another hook", "labelKey", k, "oldValue", old, "newValue", v, "fileName", fileName)
			}
			features[k] = v
		}
	}

	return features, nil
}

// Run one hook
func runHook(file string) ([][]byte, error) {
	var lines [][]byte

	path := filepath.Join(hookDir, file)
	filestat, err := os.Stat(path)
	if err != nil {
		klog.ErrorS(err, "failed to get filestat, skipping hook", "path", path)
		return lines, err
	}

	if filestat.Mode().IsRegular() {
		cmd := exec.Command(path)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Run hook
		err = cmd.Run()

		// Forward stderr to our logger
		errLines := bytes.Split(stderr.Bytes(), []byte("\n"))
		for i, line := range errLines {
			if i == len(errLines)-1 && len(line) == 0 {
				// Don't print the last empty string
				break
			}
			klog.InfoS(fmt.Sprintf("%s: %s", file, line))
		}

		// Do not return any lines if an error occurred
		if err != nil {
			return lines, err
		}
		lines = bytes.Split(stdout.Bytes(), []byte("\n"))
	}

	return lines, nil
}

// Read all files to get features
func getFeaturesFromFiles() (map[string]string, error) {
	features := make(map[string]string)

	files, err := os.ReadDir(featureFilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			klog.InfoS("features directory does not exist", "path", featureFilesDir)
			return features, nil
		}
		return features, fmt.Errorf("unable to access %v: %v", featureFilesDir, err)
	}

	for _, file := range files {
		fileName := file.Name()
		lines, err := getFileContent(fileName)
		if err != nil {
			klog.ErrorS(err, "failed to read file", "fileName", fileName)
			continue
		}

		// Append features
		fileFeatures := parseFeatures(lines)
		klog.V(4).InfoS("feature file read", "fileName", fileName, "features", utils.DelayedDumper(fileFeatures))
		for k, v := range fileFeatures {
			if old, ok := features[k]; ok {
				klog.InfoS("overriding label value from another feature file", "labelKey", k, "oldValue", old, "newValue", v, "fileName", fileName)
			}
			features[k] = v
		}
	}

	return features, nil
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
		fileContent, err := os.ReadFile(path)

		// Do not return any lines if an error occurred
		if err != nil {
			return lines, err
		}
		lines = bytes.Split(fileContent, []byte("\n"))
	}

	return lines, nil
}

func init() {
	source.Register(&src)
}
