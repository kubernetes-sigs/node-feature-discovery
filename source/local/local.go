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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Name of this feature source
const Name = "local"

const LabelFeature = "label"

// Config
var (
	featureFilesDir = "/etc/kubernetes/node-feature-discovery/features.d/"
	hookDir         = "/etc/kubernetes/node-feature-discovery/source.d/"
)

// localSource implements the FeatureSource and LabelSource interfaces.
type localSource struct {
	features *feature.DomainFeatures
}

// Singleton source instance
var (
	src localSource
	_   source.FeatureSource = &src
	_   source.LabelSource   = &src
)

// Name method of the LabelSource interface
func (s *localSource) Name() string { return Name }

// Priority method of the LabelSource interface
func (s *localSource) Priority() int { return 20 }

// GetLabels method of the LabelSource interface
func (s *localSource) GetLabels() (source.FeatureLabels, error) {
	labels := make(source.FeatureLabels)
	features := s.GetFeatures()

	for k, v := range features.Values[LabelFeature].Elements {
		labels[k] = v
	}
	return labels, nil
}

// Discover method of the FeatureSource interface
func (s *localSource) Discover() error {
	s.features = feature.NewDomainFeatures()

	featuresFromHooks, err := getFeaturesFromHooks()
	if err != nil {
		klog.Error(err)
	}

	featuresFromFiles, err := getFeaturesFromFiles()
	if err != nil {
		klog.Error(err)
	}

	// Merge features from hooks and files
	for k, v := range featuresFromHooks {
		if old, ok := featuresFromFiles[k]; ok {
			klog.Warningf("overriding '%s': value changed from '%s' to '%s'",
				k, old, v)
		}
		featuresFromFiles[k] = v
	}
	s.features.Values[LabelFeature] = feature.NewValueFeatures(featuresFromFiles)

	utils.KlogDump(3, "discovered local features:", "  ", s.features)

	return nil
}

// GetFeatures method of the FeatureSource Interface
func (s *localSource) GetFeatures() *feature.DomainFeatures {
	if s.features == nil {
		s.features = feature.NewDomainFeatures()
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

	files, err := ioutil.ReadDir(hookDir)
	if err != nil {
		if os.IsNotExist(err) {
			klog.Infof("hook directory %v does not exist", hookDir)
			return features, nil
		}
		return features, fmt.Errorf("unable to access %v: %v", hookDir, err)
	}

	for _, file := range files {
		fileName := file.Name()
		lines, err := runHook(fileName)
		if err != nil {
			klog.Errorf("source local failed running hook '%v': %v", fileName, err)
			continue
		}

		// Append features
		fileFeatures := parseFeatures(lines)
		utils.KlogDump(4, fmt.Sprintf("features from hook %q:", fileName), "  ", fileFeatures)
		for k, v := range fileFeatures {
			if old, ok := features[k]; ok {
				klog.Warningf("overriding label '%s' from another hook (%s): value changed from '%s' to '%s'",
					k, fileName, old, v)
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
		klog.Errorf("skipping %v, failed to get stat: %v", path, err)
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
			klog.Errorf("%v: %s", file, line)
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

	files, err := ioutil.ReadDir(featureFilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			klog.Infof("features directory %v does not exist", featureFilesDir)
			return features, nil
		}
		return features, fmt.Errorf("unable to access %v: %v", featureFilesDir, err)
	}

	for _, file := range files {
		fileName := file.Name()
		lines, err := getFileContent(fileName)
		if err != nil {
			klog.Errorf("source local failed reading file '%v': %v", fileName, err)
			continue
		}

		// Append features
		fileFeatures := parseFeatures(lines)
		utils.KlogDump(4, fmt.Sprintf("features from feature file %q:", fileName), "  ", fileFeatures)
		for k, v := range fileFeatures {
			if old, ok := features[k]; ok {
				klog.Warningf("overriding label '%s' from another features.d file (%s): value changed from '%s' to '%s'",
					k, fileName, old, v)
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
		klog.Errorf("skipping %v, failed to get stat: %v", path, err)
		return lines, err
	}

	if filestat.Mode().IsRegular() {
		fileContent, err := ioutil.ReadFile(path)

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
