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

package system

import (
	"bufio"
	"log"
	"os"
	"regexp"
	"strings"

	"sigs.k8s.io/node-feature-discovery/source"
)

var osReleaseFields = [...]string{
	"ID",
	"VERSION_ID",
}

// Implement FeatureSource interface
type Source struct{}

func (s Source) Name() string { return "system" }

func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	release, err := parseOSRelease()
	if err != nil {
		log.Printf("ERROR: failed to get os-release: %s", err)
	} else {
		for _, key := range osReleaseFields {
			if value, exists := release[key]; exists {
				feature := "os_release." + key
				features[feature] = value

				if key == "VERSION_ID" {
					versionComponents := splitVersion(value)
					for subKey, subValue := range versionComponents {
						features[feature+"."+subKey] = subValue
					}
				}
			}
		}
	}
	return features, nil
}

// Read and parse os-release file
func parseOSRelease() (map[string]string, error) {
	release := map[string]string{}

	f, err := os.Open("/host-etc/os-release")
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`^(?P<key>\w+)=(?P<value>.+)`)

	// Read line-by-line
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if m := re.FindStringSubmatch(line); m != nil {
			release[m[1]] = strings.Trim(m[2], `"`)
		}
	}

	return release, nil
}

// Split version number into sub-components. Verifies that they are numerical
// so that they can be fully utilized in k8s nodeAffinity
func splitVersion(version string) map[string]string {
	components := map[string]string{}
	// Currently, split out just major and minor version
	re := regexp.MustCompile(`^(?P<major>\d+)(\.(?P<minor>\d+))?(\..*)?$`)
	if m := re.FindStringSubmatch(version); m != nil {
		for i, name := range re.SubexpNames() {
			if i != 0 && name != "" {
				components[name] = m[i]
			}
		}
	}
	return components
}
