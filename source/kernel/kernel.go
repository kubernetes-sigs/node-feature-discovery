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

package kernel

import (
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/node-feature-discovery/source"
)

// Implement FeatureSource interface
type Source struct{}

func (s Source) Name() string { return "kernel" }

func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	// Read kernel version
	version, err := parseVersion()
	if err != nil {
		glog.Errorf("Failed to get kernel version: %v", err)
	} else {
		for key := range version {
			features["version."+key] = version[key]
		}
	}
	return features, nil
}

// Read and parse kernel version
func parseVersion() (map[string]string, error) {
	version := map[string]string{}

	// Open file for reading
	raw, err := ioutil.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return nil, err
	}

	full := strings.TrimSpace(string(raw))
	version["full"] = full

	// Regexp for parsing version components
	re := regexp.MustCompile(`^(?P<major>\d+)(\.(?P<minor>\d+))?(\.(?P<revision>\d+))?(-.*)?$`)
	if m := re.FindStringSubmatch(full); m != nil {
		for i, name := range re.SubexpNames() {
			if i != 0 && name != "" {
				version[name] = m[i]
			}
		}
	}

	return version, nil
}
