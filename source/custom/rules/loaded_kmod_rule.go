/*
Copyright 2020 The Kubernetes Authors.

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

package rules

import (
	"fmt"
	"io/ioutil"
	"strings"
)

// Rule that matches on loaded kernel modules in the system
type LoadedKModRule []string

const kmodProcfsPath = "/proc/modules"

// Match loaded kernel modules on provided list of kernel modules
func (kmods *LoadedKModRule) Match() (bool, error) {
	loadedModules, err := kmods.getLoadedModules()
	if err != nil {
		return false, fmt.Errorf("failed to get loaded kernel modules. %s", err.Error())
	}
	for _, kmod := range *kmods {
		if _, ok := loadedModules[kmod]; !ok {
			// kernel module not loaded
			return false, nil
		}
	}
	return true, nil
}

func (kmods *LoadedKModRule) getLoadedModules() (map[string]struct{}, error) {
	out, err := ioutil.ReadFile(kmodProcfsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %s", kmodProcfsPath, err.Error())
	}

	loadedMods := make(map[string]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		// skip empty lines
		if len(line) == 0 {
			continue
		}
		// append loaded module
		loadedMods[strings.Fields(line)[0]] = struct{}{}
	}
	return loadedMods, nil
}
