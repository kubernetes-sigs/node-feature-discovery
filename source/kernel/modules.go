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

func getLoadedModules() ([]string, error) {
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
		// append loaded module
		loadedMods = append(loadedMods, strings.Fields(line)[0])
	}
	return loadedMods, nil
}

func getBuiltinModules() ([]string, error) {
	kVersion, err := getVersion()
	if err != nil {
		return []string{}, err
	}

	kBuiltinModPath := hostpath.LibDir.Path("modules/" + kVersion + "/modules.builtin")
	out, err := os.ReadFile(kBuiltinModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %s", kBuiltinModPath, err.Error())
	}

	return parseBuiltinModules(string(out)), nil
}

// parseBuiltinModules returns the names of the modules listed in a
// modules.builtin file. modules.builtin lists object file paths, which may
// contain dashes (e.g. kernel/drivers/md/dm-mod.ko), but the kernel always
// names a module with underscores (dm_mod in /proc/modules and /sys/module).
// Convert dashes so that the same module has the same name whether it is
// loaded or built in.
func parseBuiltinModules(content string) []string {
	lines := strings.Split(content, "\n")
	builtinMods := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasSuffix(line, ".ko") {
			continue
		}

		name := strings.TrimSuffix(path.Base(line), ".ko")
		builtinMods = append(builtinMods, strings.ReplaceAll(name, "-", "_"))
	}
	return builtinMods
}
