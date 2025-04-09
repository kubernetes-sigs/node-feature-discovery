/*
Copyright 2025 The Kubernetes Authors.

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
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

func getCommonLibcPaths() []string {
	return []string{
		// glibc
		hostpath.UsrDir.Path("lib64/libc.so.6"),
		hostpath.LibDir.Path("x86_64-linux-gnu/libc.so.6"),
		hostpath.LibDir.Path("libc.so.6"),

		// musl
		hostpath.LibDir.Path("ld-musl-x86_64.so.1"),
	}
}
