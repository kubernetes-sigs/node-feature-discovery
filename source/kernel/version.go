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

package kernel

import (
	"os"
	"regexp"
	"strings"
)

// Read and parse kernel version
func discoverVersion() (map[string]string, error) {
	raw, err := getVersion()
	if err != nil {
		return nil, err
	}

	return parseVersion(raw), nil
}

func parseVersion(raw string) map[string]string {
	version := map[string]string{}

	// Replace forbidden symbols
	fullRegex := regexp.MustCompile("[^-A-Za-z0-9_.]")
	full := fullRegex.ReplaceAllString(raw, "_")
	// Label values must start and end with an alphanumeric
	full = strings.Trim(full, "-_.")

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

	return version
}

func getVersion() (string, error) {
	unameRaw, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(unameRaw)), nil
}
