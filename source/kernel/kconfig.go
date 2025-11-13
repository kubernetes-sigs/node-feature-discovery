/*
Copyright 2018-2021 The Kubernetes Authors.

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
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

// Read gzipped kernel config
func readKconfigGzip(filename string) ([]byte, error) {
	// Open file for reading
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close() // nolint: errcheck

	// Uncompress data
	r, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer r.Close() // nolint: errcheck

	return io.ReadAll(r)
}

// parseKconfig reads Linux kernel configuration and returns all set options
// and their values. It returns two copies of the parsed options: one with
// values exactly as they are presented in the kernel configuration file (with
// the exception that leading and trailing quotes are stripped) and one where
// '=y' and '=m' are converted to 'true'.
func parseKconfig(configPath string) (realKconfig, legacyKconfig map[string]string, err error) {
	realKconfig = map[string]string{}
	legacyKconfig = map[string]string{}

	raw := []byte(nil)
	var searchPaths []string

	kVer, err := getVersion()
	if err != nil {
		searchPaths = []string{
			"/proc/config.gz",
			hostpath.UsrDir.Path("src/linux/.config"),
		}
	} else {
		// from k8s.io/system-validator used by kubeadm
		// preflight checks
		searchPaths = []string{
			"/proc/config.gz",
			hostpath.UsrDir.Path("src/linux-" + kVer + "/.config"),
			hostpath.UsrDir.Path("src/linux-rt-" + kVer + "/.config"),
			hostpath.UsrDir.Path("src/linux/.config"),
			hostpath.UsrDir.Path("lib/modules/" + kVer + "/config"),
			hostpath.UsrDir.Path("lib/ostree-boot/config-" + kVer),
			hostpath.UsrDir.Path("lib/kernel/config-" + kVer),
			hostpath.UsrDir.Path("src/linux-headers-" + kVer + "/.config"),
			"/lib/modules/" + kVer + "/build/.config",
			hostpath.UsrDir.Path("src/linux-rt-headers-" + kVer + "/.config"),
			hostpath.BootDir.Path("config-" + kVer),
		}
	}

	for _, path := range append([]string{configPath}, searchPaths...) {
		if len(path) > 0 {
			if filepath.Ext(path) == ".gz" {
				if raw, err = readKconfigGzip(path); err == nil {
					break
				}
			} else {
				if raw, err = os.ReadFile(path); err == nil {
					break
				}
			}
		}
	}

	if raw == nil {
		return nil, nil, fmt.Errorf("failed to read kernel config from %+v", append([]string{configPath}, searchPaths...))
	}

	// Process data, line-by-line
	lines := bytes.SplitSeq(raw, []byte("\n"))
	for line := range lines {
		str := string(line)
		if strings.HasPrefix(str, "CONFIG_") {
			split := strings.SplitN(str, "=", 2)
			if len(split) != 2 {
				continue
			}
			// Trim the "CONFIG_" prefix
			name := split[0][7:]
			value := strings.Trim(split[1], `"`)

			realKconfig[name] = value

			// Provide the "mangled" kconfig values for backwards compatibility
			if split[1] == "y" || split[1] == "m" {
				legacyKconfig[name] = "true"
			} else {
				legacyKconfig[name] = value
			}
		}
	}

	return realKconfig, legacyKconfig, nil
}
