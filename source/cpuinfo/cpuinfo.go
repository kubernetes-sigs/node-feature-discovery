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

package cpuinfo

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/source"
)

var cpuReleaseFields = [...]string{
	"model",
	"cpu_MHz",
	"cpu_family",
	"vendor_id",
	"stepping",
}

const Name = "cpuinfo"

// systemSource implements the LabelSource interface.
type systemSource struct{}

// Singleton source instance
var (
	src systemSource
	_   source.LabelSource = &src
)

func (s *systemSource) Name() string { return Name }

// Priority method of the LabelSource interface
func (s *systemSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *systemSource) GetLabels() (source.FeatureLabels, error) {
	features := source.FeatureLabels{}

	info, err := parseCPUInfo()
	if err != nil {
		klog.Errorf("failed to get cpu-info: %s", err)
	} else {
		for _, key := range cpuReleaseFields {
			if value, exists := info[key]; exists {
				feature := "cpuinfo." + key
				features[feature] = value
			}
		}
	}
	return features, nil
}

// Read and parse cpuinfo file
func parseCPUInfo() (map[string]string, error) {
	release := map[string]string{}

	f, err := os.Open(source.ProcDir.Path("cpuinfo"))
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`^(?P<key>\w.+):(?P<value>.+)`)

	// Read line-by-line
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if m := re.FindStringSubmatch(line); m != nil {
			m[1] = strings.TrimSpace(m[1])
			m[1] = strings.ReplaceAll(m[1], " ", "_")
			m[2] = strings.TrimSpace(m[2])
			release[m[1]] = strings.Trim(m[2], `"`)
		}
	}

	return release, nil
}

func init() {
	source.Register(&src)
}
