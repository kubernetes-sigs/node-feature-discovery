/*
Copyright 2017-2018 The Kubernetes Authors.

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
	"os"
	"path/filepath"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

// SelinuxEnabled detects if selinux has been enabled in the kernel
func SelinuxEnabled() (bool, error) {
	sysfsBase := hostpath.SysfsDir.Path("fs")
	if _, err := os.Stat(sysfsBase); err != nil {
		return false, err
	}

	selinuxBase := filepath.Join(sysfsBase, "selinux")
	if _, err := os.Stat(selinuxBase); os.IsNotExist(err) {
		klog.V(1).InfoS("selinux not available on the system")
		return false, nil
	}

	status, err := os.ReadFile(filepath.Join(selinuxBase, "enforce"))
	if err != nil {
		return false, err
	}
	if status[0] == byte('1') {
		// selinux is enabled.
		return true, nil
	}
	return false, nil
}
