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

	"github.com/golang/glog"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Source implements FeatureSource.
type Source struct{}

// Name returns an identifier string for this feature source.
func (s Source) Name() string { return "rdt" }

// Discover returns feature names for CMT and CAT if supported.
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	cmd := exec.Command("bash", "-c", "mon-discovery")
	if err := cmd.Run(); err != nil {
		glog.Errorf("support for RDT monitoring was not detected: %v", err)
	} else {
		// RDT monitoring detected.
		features["RDTMON"] = true
	}

	cmd = exec.Command("bash", "-c", "mon-cmt-discovery")
	if err := cmd.Run(); err != nil {
		glog.Errorf("support for RDT CMT monitoring was not detected: %v", err)
	} else {
		// RDT CMT monitoring detected.
		features["RDTCMT"] = true
	}

	cmd = exec.Command("bash", "-c", "mon-mbm-discovery")
	if err := cmd.Run(); err != nil {
		glog.Errorf("support for RDT MBM monitoring was not detected: %v", err)
	} else {
		// RDT MBM monitoring detected.
		features["RDTMBM"] = true
	}

	cmd = exec.Command("bash", "-c", "l3-alloc-discovery")
	if err := cmd.Run(); err != nil {
		glog.Errorf("support for RDT L3 allocation was not detected: %v", err)
	} else {
		// RDT L3 cache allocation detected.
		features["RDTL3CA"] = true
	}

	cmd = exec.Command("bash", "-c", "l2-alloc-discovery")
	if err := cmd.Run(); err != nil {
		glog.Errorf("support for RDT L2 allocation was not detected: %v", err)
	} else {
		// RDT L2 cache allocation detected.
		features["RDTL2CA"] = true
	}

	cmd = exec.Command("bash", "-c", "mem-bandwidth-alloc-discovery")
	if err := cmd.Run(); err != nil {
		glog.Errorf("support for RDT Memory bandwidth allocation was not detected: %v", err)
	} else {
		// RDT Memory bandwidth allocation detected.
		features["RDTMBA"] = true
	}

	return features, nil
}
