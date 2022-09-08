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
	klog.V(1).Infof("all configmap based custom feature specs: %+v", features)
	return features
}

func readDir(dirName string, recursive bool) []CustomRule {
	features := make([]CustomRule, 0)

	klog.V(1).Infof("getting files in %s", dirName)
	files, err := os.ReadDir(dirName)
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(1).Infof("custom config directory %q does not exist", dirName)
		} else {
			klog.Errorf("unable to access custom config directory %q, %v", dirName, err)
		}
		return features
	}

	for _, file := range files {
		fileName := filepath.Join(dirName, file.Name())

		if file.IsDir() {
			if recursive {
				klog.V(1).Infof("processing dir %q", fileName)
				features = append(features, readDir(fileName, false)...)
			} else {
				klog.V(2).Infof("skipping dir %q", fileName)
			}
			continue
		}
		if strings.HasPrefix(file.Name(), ".") {
			klog.V(2).Infof("skipping hidden file %q", fileName)
			continue
		}
		klog.V(2).Infof("processing file %q", fileName)

		bytes, err := os.ReadFile(fileName)
		if err != nil {
			klog.Errorf("could not read custom config file %q, %v", fileName, err)
			continue
		}
		klog.V(2).Infof("custom config rules raw: %s", string(bytes))

		config := &[]CustomRule{}
		err = yaml.UnmarshalStrict(bytes, config)
		if err != nil {
			klog.Errorf("could not parse custom config file %q, %v", fileName, err)
			continue
		}

		features = append(features, *config...)
	}
	return features
}
