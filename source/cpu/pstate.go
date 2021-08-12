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

package cpu

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/source"
)

// Discover p-state related features such as turbo boost.
func detectPstate() (map[string]string, error) {
	// On other platforms, the frequency boost mechanism is software-based.
	// So skip pstate detection on other architectures.
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "386" {
		return nil, nil
	}

	// Check that sysfs is available
	sysfsBase := source.SysfsDir.Path("devices/system/cpu")
	if _, err := os.Stat(sysfsBase); err != nil {
		return nil, fmt.Errorf("unable to detect pstate status: %w", err)
	}
	pstateDir := filepath.Join(sysfsBase, "intel_pstate")
	if _, err := os.Stat(pstateDir); os.IsNotExist(err) {
		klog.V(1).Info("intel pstate driver not enabled")
		return nil, nil
	}

	// Get global pstate status
	data, err := ioutil.ReadFile(filepath.Join(pstateDir, "status"))
	if err != nil {
		return nil, fmt.Errorf("could not read pstate status: %w", err)
	}
	status := strings.TrimSpace(string(data))
	if status == "off" {
		// No need to check other pstate features
		klog.Infof("intel_pstate driver is not in use")
		return nil, nil
	}
	features := map[string]string{"status": status}

	// Check turbo boost
	bytes, err := ioutil.ReadFile(filepath.Join(pstateDir, "no_turbo"))
	if err != nil {
		klog.Errorf("can't detect whether turbo boost is enabled: %s", err.Error())
	} else {
		features["turbo"] = "false"
		if bytes[0] == byte('0') {
			features["turbo"] = "true"
		}
	}

	if status != "active" {
		// Don't check other features which depend on active state
		return features, nil
	}

	// Determine scaling governor that is being used
	cpufreqDir := filepath.Join(sysfsBase, "cpufreq")
	policies, err := ioutil.ReadDir(cpufreqDir)
	if err != nil {
		klog.Errorf("failed to read cpufreq directory: %s", err.Error())
		return features, nil
	}

	scaling := ""
	for _, policy := range policies {
		// Ensure at least one cpu is using this policy
		cpus, err := ioutil.ReadFile(filepath.Join(cpufreqDir, policy.Name(), "affected_cpus"))
		if err != nil {
			klog.Errorf("could not read cpufreq policy %s affected_cpus", policy.Name())
			continue
		}
		if strings.TrimSpace(string(cpus)) == "" {
			klog.Infof("policy %s has no associated cpus", policy.Name())
			continue
		}

		data, err := ioutil.ReadFile(filepath.Join(cpufreqDir, policy.Name(), "scaling_governor"))
		if err != nil {
			klog.Errorf("could not read cpufreq policy %s scaling_governor", policy.Name())
			continue
		}
		policy_scaling := strings.TrimSpace(string(data))
		// Check that all of the policies have the same scaling governor, if not don't set feature
		if scaling != "" && scaling != policy_scaling {
			klog.Infof("scaling_governor for policy %s doesn't match prior policy", policy.Name())
			scaling = ""
			break
		}
		scaling = policy_scaling
	}

	if scaling != "" {
		features["scaling_governor"] = scaling
	}

	return features, nil
}
