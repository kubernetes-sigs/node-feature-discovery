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

	"github.com/kubernetes-incubator/node-feature-discovery/source"
)

// Config
var (
	hookDir = "/etc/kubernetes/node-feature-discovery/source.d/"
	logger  = log.New(os.Stderr, "", log.LstdFlags)
)

// Implement FeatureSource interface
type Source struct{}

func (s Source) Name() string { return "local" }

func (s Source) Discover() (source.Features, error) {
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
		hook := file.Name()
		hookFeatures, err := runHook(hook)
		if err != nil {
			log.Printf("ERROR: source hook '%v' failed: %v", hook, err)
			continue
		}
		for feature, value := range hookFeatures {
			if feature[0] == '/' {
				// Use feature name as the label as is if it is prefixed with a slash
				features[feature[1:]] = value
			} else {
				// Normally, use hook name as label prefix
				features[hook+"-"+feature] = value
			}
		}
	}

	return features, nil
}

// Run one hook
func runHook(file string) (map[string]string, error) {
	features := map[string]string{}

	path := filepath.Join(hookDir, file)
	filestat, err := os.Stat(path)
	if err != nil {
		log.Printf("ERROR: skipping %v, failed to get stat: %v", path, err)
		return features, err
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
		lines := bytes.Split(stderr.Bytes(), []byte("\n"))
		for i, line := range lines {
			if i == len(lines)-1 && len(line) == 0 {
				// Don't print the last empty string
				break
			}
			log.Printf("%v: %s", file, line)
		}

		// Do not return any features if an error occurred
		if err != nil {
			return features, err
		}

		// Return features printed to stdout
		lines = bytes.Split(stdout.Bytes(), []byte("\n"))
		for _, line := range lines {
			if len(line) > 0 {
				lineSplit := strings.SplitN(string(line), "=", 2)
				if len(lineSplit) == 1 {
					features[lineSplit[0]] = "true"
				} else {
					features[lineSplit[0]] = lineSplit[1]
				}
			}
		}
	}

	return features, nil
}
