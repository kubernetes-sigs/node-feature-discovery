/*
Copyright 2017 The Kubernetes Authors.

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

package rdt

import (
	"os/exec"
	"path"

	"github.com/golang/glog"
)

const (
	// RDTBin is the path to RDT detection helpers.
	RDTBin = "/go/src/k8s.io/node-feature-discovery/rdt-discovery"
)

// Source implements FeatureSource.
type Source struct{}

func (s Source) Name() string { return "rdt" }

// Returns feature names for CMT and CAT if suppported.
func (s Source) Discover() ([]string, error) {
	features := []string{}

	cmd := exec.Command("bash", "-c", path.Join(RDTBin, "mon-discovery"))
	if err := cmd.Run(); err != nil {
		glog.Errorf("support for RDT monitoring was not detected: %v", err)
	} else {
		// RDT monitoring detected.
		features = append(features, "RDTMON")
	}

	cmd = exec.Command("bash", "-c", path.Join(RDTBin, "l3-alloc-discovery"))
	if err := cmd.Run(); err != nil {
		glog.Errorf("support for RDT L3 allocation was not detected: %v", err)
	} else {
		// RDT L3 cache allocation detected.
		features = append(features, "RDTL3CA")
	}

	cmd = exec.Command("bash", "-c", path.Join(RDTBin, "l2-alloc-discovery"))
	if err := cmd.Run(); err != nil {
		glog.Errorf("support for RDT L2 allocation was not detected: %v", err)
	} else {
		// RDT L2 cache allocation detected.
		features = append(features, "RDTL2CA")
	}

	return features, nil
}
