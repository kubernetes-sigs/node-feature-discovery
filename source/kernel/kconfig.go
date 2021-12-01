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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/source"
)

// Read gzipped kernel config
func readKconfigGzip(filename string) ([]byte, error) {
	// Open file for reading
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Uncompress data
	r, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return ioutil.ReadAll(r)
}

// ParseKconfig reads kconfig and return a map
func parseKconfig(configPath string) (map[string]string, error) {
	kconfig := map[string]string{}
	raw := []byte(nil)
	var err error
	var searchPaths []string

	kVer, err := getVersion()
	if err != nil {
		searchPaths = []string{
			"/proc/config.gz",
			source.UsrDir.Path("src/linux/.config"),
		}
	} else {
		// from k8s.io/system-validator used by kubeadm
		// preflight checks
		searchPaths = []string{
			"/proc/config.gz",
			source.UsrDir.Path("src/linux-" + kVer + "/.config"),
			source.UsrDir.Path("src/linux/.config"),
			source.UsrDir.Path("lib/modules/" + kVer + "/config"),
			source.UsrDir.Path("lib/ostree-boot/config-" + kVer),
			source.UsrDir.Path("lib/kernel/config-" + kVer),
			source.UsrDir.Path("src/linux-headers-" + kVer + "/.config"),
			"/lib/modules/" + kVer + "/build/.config",
			source.BootDir.Path("config-" + kVer),
		}
	}

	for _, path := range append([]string{configPath}, searchPaths...) {
		if len(path) > 0 {
			if ".gz" == filepath.Ext(path) {
				if raw, err = readKconfigGzip(path); err == nil {
					break
				}
			} else {
				if raw, err = ioutil.ReadFile(path); err == nil {
					break
				}
			}
		}
	}

	if raw == nil {
		return nil, fmt.Errorf("failed to read kernel config from %+v", append([]string{configPath}, searchPaths...))
	}

	// Process data, line-by-line
	lines := bytes.Split(raw, []byte("\n"))
	for _, line := range lines {
		str := string(line)
		if strings.HasPrefix(str, "CONFIG_") {
			split := strings.SplitN(str, "=", 2)
			if len(split) != 2 {
				continue
			}
			// Trim the "CONFIG_" prefix
			name := split[0][7:]

			if split[1] == "y" || split[1] == "m" {
				kconfig[name] = "true"
			} else {
				value := strings.Trim(split[1], `"`)
				if len(value) > validation.LabelValueMaxLength {
					klog.Warningf("ignoring kconfig option '%s': value exceeds max length of %d characters", name, validation.LabelValueMaxLength)
					continue
				}
				kconfig[name] = value
			}
		}
	}

	return kconfig, nil
}
