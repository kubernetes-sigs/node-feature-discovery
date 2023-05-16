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
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

// Discover if c-states are enabled
func detectCstate() (map[string]string, error) {
	cstate := make(map[string]string)

	// Check that sysfs is available
	sysfsBase := hostpath.SysfsDir.Path("devices/system/cpu")
	if _, err := os.Stat(sysfsBase); err != nil {
		return cstate, fmt.Errorf("unable to detect cstate status: %w", err)
	}
	cpuidleDir := filepath.Join(sysfsBase, "cpuidle")
	if _, err := os.Stat(cpuidleDir); os.IsNotExist(err) {
		klog.V(1).InfoS("cpuidle disabled in the kernel")
		return cstate, nil
	}

	// When the intel_idle driver is in use (default), check setting of max_cstates
	driver, err := os.ReadFile(filepath.Join(cpuidleDir, "current_driver"))
	if err != nil {
		return cstate, fmt.Errorf("cannot get driver for cpuidle: %w", err)
	}

	if d := strings.TrimSpace(string(driver)); d != "intel_idle" {
		// Currently only checking intel_idle driver for cstates
		klog.V(1).InfoS("intel_idle driver is not in use", "currentIdleDriver", d)
		return cstate, nil
	}

	data, err := os.ReadFile(hostpath.SysfsDir.Path("module/intel_idle/parameters/max_cstate"))
	if err != nil {
		return cstate, fmt.Errorf("cannot determine cstate from max_cstates: %w", err)
	}
	cstates, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return cstate, fmt.Errorf("non-integer value of cstates: %w", err)
	}

	cstate["enabled"] = strconv.FormatBool(cstates > 0)

	return cstate, nil
}
