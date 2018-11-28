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

package selinux

import (
	"fmt"
	"io/ioutil"

	"sigs.k8s.io/node-feature-discovery/source"
)

type Source struct{}

func (s Source) Name() string { return "selinux" }

func (s Source) Discover() (source.Features, error) {
	features := source.Features{}
	status, err := ioutil.ReadFile("/host-sys/fs/selinux/enforce")
	if err != nil {
		return nil, fmt.Errorf("Failed to detect the status of selinux, please check if the system supports selinux and make sure /sys on the host is mounted into the container: %s", err.Error())
	}
	if status[0] == byte('1') {
		// selinux is enabled.
		features["enabled"] = true
	}
	return features, nil
}
