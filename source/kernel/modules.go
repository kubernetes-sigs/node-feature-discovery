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
	"strings"
)

const kmodProcfsPath = "/proc/modules"

func getLoadedModules() ([]string, error) {
	out, err := os.ReadFile(kmodProcfsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %s", kmodProcfsPath, err.Error())
	}

	lines := strings.Split(string(out), "\n")
	loadedMods := make([]string, len(lines))
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
