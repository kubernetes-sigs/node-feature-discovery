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

package custom

import (
	"os"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

// Directory stores the full path for the custom sources folder
const Directory = "/etc/kubernetes/node-feature-discovery/custom.d"

// getDirectoryFeatureConfig returns features configured in the "/etc/kubernetes/node-feature-discovery/custom.d"
// host directory and its 1st level subdirectories, which can be populated e.g. by ConfigMaps
func getDirectoryFeatureConfig() []CustomRule {
	features := readDir(Directory, true)
	klog.V(3).InfoS("all custom feature specs from config dir", "featureSpecs", features)
	return features
}

func readDir(dirName string, recursive bool) []CustomRule {
	features := make([]CustomRule, 0)

	klog.V(4).InfoS("reading directory", "path", dirName)
	files, err := os.ReadDir(dirName)
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(4).InfoS("directory does not exist", "path", dirName)
		} else {
			klog.ErrorS(err, "unable to access directory", "path", dirName)
		}
		return features
	}

	for _, file := range files {
		fileName := filepath.Join(dirName, file.Name())

		if file.IsDir() {
			if recursive {
				klog.V(4).InfoS("processing directory", "path", fileName)
				features = append(features, readDir(fileName, false)...)
			} else {
				klog.V(4).InfoS("skipping directory", "path", fileName)
			}
			continue
		}
		if strings.HasPrefix(file.Name(), ".") {
			klog.V(4).InfoS("skipping hidden file", "path", fileName)
			continue
		}
		klog.V(4).InfoS("processing file", "path", fileName)

		bytes, err := os.ReadFile(fileName)
		if err != nil {
			klog.ErrorS(err, "could not read file", "path", fileName)
			continue
		}

		config := &[]CustomRule{}
		err = yaml.UnmarshalStrict(bytes, config)
		if err != nil {
			klog.ErrorS(err, "could not parse file", "path", fileName)
			continue
		}

		features = append(features, *config...)
	}
	return features
}
