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

package kernel

import (
	"fmt"
	"os"
	"path"
	"strings"

	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

const kmodProcfsPath = "/proc/modules"

func getLoadedModules(s *kernelSource) ([]string, error) {
	out, err := os.ReadFile(kmodProcfsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %s", kmodProcfsPath, err.Error())
	}

	lines := strings.Split(string(out), "\n")
	loadedMods := make([]string, 0, len(lines))
	for _, line := range lines {
		// skip empty lines
		if len(line) == 0 {
			continue
		}

		mod := strings.Fields(line)[0]
		// Skip if module doesn't match ModuleWhiteList
		if !s.moduleWhiteList.MatchString(mod) {
			continue
		}

		// append loaded module
		loadedMods = append(loadedMods, mod)
	}
	return loadedMods, nil
}

func getBuiltinModules(s *kernelSource) ([]string, error) {
	kVersion, err := getVersion()
	if err != nil {
		return []string{}, err
	}

	kBuiltinModPath := hostpath.LibDir.Path("modules/" + kVersion + "/modules.builtin")
	out, err := os.ReadFile(kBuiltinModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %s", kBuiltinModPath, err.Error())
	}

	lines := strings.Split(string(out), "\n")
	builtinMods := make([]string, 0, len(lines))
	for _, line := range lines {
		// skip empty lines
		line = strings.TrimSpace(line)
		if !strings.HasSuffix(line, ".ko") {
			continue
		}

		mod := strings.TrimSuffix(path.Base(line), ".ko")
		// Skip if module doesn't match ModuleWhiteList
		if !s.moduleWhiteList.MatchString(mod) {
			continue
		}

		// append loaded module
		builtinMods = append(builtinMods, mod)
	}
	return builtinMods, nil
}
