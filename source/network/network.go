/*
Copyright 2017 The Kubernetes Authors.

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

package network

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"

	"sigs.k8s.io/node-feature-discovery/source"
)

// Linux net iface flags (we only specify the first few)
const (
	flagUp = 1 << iota
	_      // flagBroadcast
	_      // flagDebug
	flagLoopback
)

const sysfsBaseDir = "class/net"

// Source implements FeatureSource.
type Source struct{}

// Name returns an identifier string for this feature source.
func (s Source) Name() string { return "network" }

// NewConfig method of the FeatureSource interface
func (s *Source) NewConfig() source.Config { return nil }

// GetConfig method of the FeatureSource interface
func (s *Source) GetConfig() source.Config { return nil }

// SetConfig method of the FeatureSource interface
func (s *Source) SetConfig(source.Config) {}

// Discover returns feature names sriov-configured and sriov if SR-IOV capable NICs are present and/or SR-IOV virtual functions are configured on the node
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	netInterfaces, err := ioutil.ReadDir(source.SysfsDir.Path(sysfsBaseDir))
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %s", err.Error())
	}

	// iterating through network interfaces to obtain their respective number of virtual functions
	for _, netInterface := range netInterfaces {
		name := netInterface.Name()
		flags, err := readIfFlags(name)
		if err != nil {
			log.Printf("%v", err)
			continue
		}

		if flags&flagUp != 0 && flags&flagLoopback == 0 {
			totalBytes, err := ioutil.ReadFile(source.SysfsDir.Path(sysfsBaseDir, name, "device/sriov_totalvfs"))
			if err != nil {
				log.Printf("SR-IOV not supported for network interface: %s: %v", name, err)
				continue
			}
			total := bytes.TrimSpace(totalBytes)
			t, err := strconv.Atoi(string(total))
			if err != nil {
				log.Printf("Error in obtaining maximum supported number of virtual functions for network interface: %s: %v", name, err)
				continue
			}
			if t > 0 {
				log.Printf("SR-IOV capability is detected on the network interface: %s", name)
				log.Printf("%d maximum supported number of virtual functions on network interface: %s", t, name)
				features["sriov.capable"] = true
				numBytes, err := ioutil.ReadFile(source.SysfsDir.Path(sysfsBaseDir, name, "device/sriov_numvfs"))
				if err != nil {
					log.Printf("SR-IOV not configured for network interface: %s: %s", name, err)
					continue
				}
				num := bytes.TrimSpace(numBytes)
				n, err := strconv.Atoi(string(num))
				if err != nil {
					log.Printf("Error in obtaining the configured number of virtual functions for network interface: %s: %v", name, err)
					continue
				}
				if n > 0 {
					log.Printf("%d virtual functions configured on network interface: %s", n, name)
					features["sriov.configured"] = true
					break
				} else if n == 0 {
					log.Printf("SR-IOV not configured on network interface: %s", name)
				}
			}
		}
	}
	return features, nil
}

func readIfFlags(name string) (uint64, error) {
	raw, err := ioutil.ReadFile(source.SysfsDir.Path(sysfsBaseDir, name, "flags"))
	if err != nil {
		return 0, fmt.Errorf("failed to read flags for interface %q: %v", name, err)
	}
	flags, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 0, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse flags for interface %q: %v", name, err)
	}

	return flags, nil
}
