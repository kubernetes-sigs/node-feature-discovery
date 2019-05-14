/*
Copyright 2018 The Kubernetes Authors.

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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sigs.k8s.io/node-feature-discovery/source"
)

// Config
var (
	featureFilesDir = "/etc/kubernetes/node-feature-discovery/features.d/"
	hookDir         = "/etc/kubernetes/node-feature-discovery/source.d/"
)

// Implement FeatureSource interface
type Source struct{}

func (s Source) Name() string { return "local" }

func (s Source) Discover() (source.Features, error) {
	featuresFromHooks, err := getFeaturesFromHooks()
	if err != nil {
		log.Printf(err.Error())
	}

	featuresFromFiles, err := getFeaturesFromFiles()
	if err != nil {
		log.Printf(err.Error())
	}

	// Merge features from hooks and files
	for k, v := range featuresFromHooks {
		if old, ok := featuresFromFiles[k]; ok {
			log.Printf("WARNING: overriding label '%s': value changed from '%s' to '%s'",
				k, old, v)
		}
		featuresFromFiles[k] = v
	}

	return featuresFromFiles, nil
}

func parseFeatures(lines [][]byte, prefix string) source.Features {
	features := source.Features{}

	for _, line := range lines {
		if len(line) > 0 {
			lineSplit := strings.SplitN(string(line), "=", 2)

			// Check if we need to add prefix
			var key string
			if strings.Contains(lineSplit[0], "/") {
				if lineSplit[0][0] == '/' {
					key = lineSplit[0][1:]
				} else {
					key = lineSplit[0]
				}
			} else {
				key = prefix + "-" + lineSplit[0]
			}

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
func getFeaturesFromHooks() (source.Features, error) {
	features := source.Features{}

	files, err := ioutil.ReadDir(hookDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("ERROR: hook directory %v does not exist", hookDir)
			return features, nil
		}
		return features, fmt.Errorf("Unable to access %v: %v", hookDir, err)
	}

	for _, file := range files {
		fileName := file.Name()
		lines, err := runHook(fileName)
		if err != nil {
			log.Printf("ERROR: source local failed running hook '%v': %v", fileName, err)
			continue
		}

		// Append features
		for k, v := range parseFeatures(lines, fileName) {
			if old, ok := features[k]; ok {
				log.Printf("WARNING: overriding label '%s' from another hook (%s): value changed from '%s' to '%s'",
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
		log.Printf("ERROR: skipping %v, failed to get stat: %v", path, err)
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
			log.Printf("%v: %s", file, line)
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
func getFeaturesFromFiles() (source.Features, error) {
	features := source.Features{}

	files, err := ioutil.ReadDir(featureFilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("ERROR: features directory %v does not exist", featureFilesDir)
			return features, nil
		}
		return features, fmt.Errorf("Unable to access %v: %v", featureFilesDir, err)
	}

	for _, file := range files {
		fileName := file.Name()
		lines, err := getFileContent(fileName)
		if err != nil {
			log.Printf("ERROR: source local failed reading file '%v': %v", fileName, err)
			continue
		}

		// Append features
		for k, v := range parseFeatures(lines, fileName) {
			if old, ok := features[k]; ok {
				log.Printf("WARNING: overriding label '%s' from another features.d file (%s): value changed from '%s' to '%s'",
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
		log.Printf("ERROR: skipping %v, failed to get stat: %v", path, err)
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
