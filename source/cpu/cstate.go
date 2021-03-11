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

package cpu

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"sigs.k8s.io/node-feature-discovery/source"
)

// Discover if c-states are enabled
func detectCstate() (bool, error) {
	// When the intel_idle driver is in use (default), check setting of max_cstates
	driver, err := ioutil.ReadFile(source.SysfsDir.Path("devices/system/cpu/cpuidle/current_driver"))
	if err != nil {
		return false, fmt.Errorf("cannot get driver for cpuidle: %s", err.Error())
	}

	if strings.TrimSpace(string(driver)) != "intel_idle" {
		// Currently only checking intel_idle driver for cstates
		return false, fmt.Errorf("intel_idle driver is not in use: %s", string(driver))
	}

	data, err := ioutil.ReadFile(source.SysfsDir.Path("module/intel_idle/parameters/max_cstate"))
	if err != nil {
		return false, fmt.Errorf("cannot determine cstate from max_cstates: %s", err.Error())
	}
	cstates, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false, fmt.Errorf("non-integer value of cstates: %s", err.Error())
	}

	return cstates > 0, nil
}
