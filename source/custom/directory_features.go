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
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

const Directory = "/etc/kubernetes/node-feature-discovery/custom.d"

// getDirectoryFeatureConfig returns features configured in the "/etc/kubernetes/node-feature-discovery/custom.d"
// host directory and its 1st level subdirectories, which can be populated e.g. by ConfigMaps
func getDirectoryFeatureConfig() []FeatureSpec {
	features := readDir(Directory, true)
	//log.Printf("DEBUG: all configmap based custom feature specs: %+v", features)
	return features
}

func readDir(dirName string, recursive bool) []FeatureSpec {
	features := make([]FeatureSpec, 0)

	log.Printf("DEBUG: getting files in %s", dirName)
	files, err := ioutil.ReadDir(dirName)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("DEBUG: custom config directory %q does not exist", dirName)
		} else {
			log.Printf("ERROR: unable to access custom config directory %q, %v", dirName, err)
		}
		return features
	}

	for _, file := range files {
		fileName := filepath.Join(dirName, file.Name())

		if file.IsDir() {
			if recursive {
				//log.Printf("DEBUG: going into dir %q", fileName)
				features = append(features, readDir(fileName, false)...)
				//} else {
				//	log.Printf("DEBUG: skipping dir %q", fileName)
			}
			continue
		}
		if strings.HasPrefix(file.Name(), ".") {
			//log.Printf("DEBUG: skipping hidden file %q", fileName)
			continue
		}
		//log.Printf("DEBUG: processing file %q", fileName)

		bytes, err := ioutil.ReadFile(fileName)
		if err != nil {
			log.Printf("ERROR: could not read custom config file %q, %v", fileName, err)
			continue
		}
		//log.Printf("DEBUG: custom config rules raw: %s", string(bytes))

		config := &[]FeatureSpec{}
		err = yaml.UnmarshalStrict(bytes, config)
		if err != nil {
			log.Printf("ERROR: could not parse custom config file %q, %v", fileName, err)
			continue
		}

		features = append(features, *config...)
	}
	return features
}
