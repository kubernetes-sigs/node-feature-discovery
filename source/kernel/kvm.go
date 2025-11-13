/*
Copyright 2017-2025 The Kubernetes Authors.

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

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

// KvmEnabled detects if kvm has been enabled in the kernel
func KvmEnabled() (bool, error) {
	_, err := os.Stat(hostpath.SysfsDir.Path("devices/virtual/misc/kvm"))
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(1).InfoS("kvm not available on the system")
			return false, nil
		}
		return false, err
	}
	return true, nil
}
