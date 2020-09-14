/*
Copyright 2018-2020 The Kubernetes Authors.

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

package kernelutils

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
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

// Read kconfig into a map
func ParseKconfig(configPath string) (map[string]string, error) {
	kconfig := map[string]string{}
	raw := []byte(nil)
	var err error
	var searchPaths []string

	kVer, err := GetKernelVersion()
	if err != nil {
		searchPaths = []string{
			"/proc/config.gz",
			"/usr/src/linux/.config",
		}
	} else {
		// from k8s.io/system-validator used by kubeadm
		// preflight checks
		searchPaths = []string{
			"/proc/config.gz",
			"/usr/src/linux-" + kVer + "/.config",
			"/usr/src/linux/.config",
			"/usr/lib/modules/" + kVer + "/config",
			"/usr/lib/ostree-boot/config-" + kVer,
			"/usr/lib/kernel/config-" + kVer,
			"/usr/src/linux-headers-" + kVer + "/.config",
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
		return nil, fmt.Errorf("Failed to read kernel config from %+v:", append([]string{configPath}, searchPaths...))
	}

	// Regexp for matching kconfig flags
	re := regexp.MustCompile(`^CONFIG_(?P<flag>\w+)=(?P<value>.+)`)

	// Process data, line-by-line
	lines := bytes.Split(raw, []byte("\n"))
	for _, line := range lines {
		if m := re.FindStringSubmatch(string(line)); m != nil {
			if m[2] == "y" || m[2] == "m" {
				kconfig[m[1]] = "true"
			} else {
				value := strings.Trim(m[2], `"`)
				if len(value) > validation.LabelValueMaxLength {
					log.Printf("WARNING: ignoring kconfig option '%s': value exceeds max length of %d characters", m[1], validation.LabelValueMaxLength)
					continue
				}
				kconfig[m[1]] = value
			}
		}
	}

	return kconfig, nil
}
