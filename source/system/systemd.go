/*
Copyright 2018 The Kubernetes Authors.

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

package system

import (
	"os/exec"
	"strings"
)

func runSystemctlCmd(args []string) (string, error) {
	output, err := exec.Command("systemctl", args...).CombinedOutput()
	return strings.TrimRight(string(output), "\n"), err
}

func SystemctlStatusIsActive(service string) (string, error) {
	args := []string{"is-active"}
	args = append(args, service)

	output, err := runSystemctlCmd(args)
	if err != nil {
		return "", err
	}

	return output, nil
}
