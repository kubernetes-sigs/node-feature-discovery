//go:build ppc64le

/*
Copyright 2023 The Kubernetes Authors.

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
	"k8s.io/klog/v2"
	"os"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
	"strconv"
)

/* Detect NX_GZIP */
func discoverCoprocessor() map[string]string {
	features := make(map[string]string)

	nxGzipPath := hostpath.SysfsDir.Path("devices/vio/ibm,compression-v1/nx_gzip_caps")

	_, err := os.Stat(nxGzipPath)
	if err != nil {
		klog.V(5).ErrorS(err, "Failed to detect nx_gzip for Nest Accelerator")
	} else {
		features["nx_gzip"] = strconv.FormatBool(true)
	}

	return features
}
