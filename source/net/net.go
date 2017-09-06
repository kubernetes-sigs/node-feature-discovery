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

package net

import (
	"net"

	"fmt"

	"io/ioutil"
	"strings"

	"strconv"

	"github.com/golang/glog"
)

var physicalInterfacePrefixes = []string{"eth", "em"}

// var instead of const just for unit test only.
var interfaceSpeedSysFilePattern = "/sys/class/net/%s/speed"

const (
	speedFeaturePattern = "speed_%d"
)

// Source implements FeatureSource.
type Source struct{}

func (s Source) Name() string { return "net" }

// Discover returns feature names for net related features such as network device bandwidth.
func (s Source) Discover() ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		glog.Errorf("get available network interfaces error. %v", err)
		return nil, fmt.Errorf("get available network interfaces error")
	}

	speedFeatures := s.getSpeedFeatures(interfaces)

	return speedFeatures, nil
}

func (s Source) getSpeedFeatures(interfaces []net.Interface) (speedFeatures []string) {
	speedFeatures = []string{}
	for _, iFace := range interfaces {
		if (iFace.Flags&net.FlagUp != 0) && (iFace.Flags&net.FlagLoopback == 0) {
			// pip stands for physical interface prefix
			for _, pip := range physicalInterfacePrefixes {
				if strings.HasPrefix(iFace.Name, pip) {
					content, err := ioutil.ReadFile(fmt.Sprintf(interfaceSpeedSysFilePattern, iFace.Name))
					if err != nil {
						glog.Errorf("read network interface speed for %s error. %v", iFace.Name, err)
						continue
					}

					contentNum := strings.TrimSuffix(string(content), "\n")
					speed, err := strconv.ParseInt(contentNum, 10, 64)
					if err != nil {
						glog.Errorf("parse network interface speed for %s error. %v", iFace.Name, err)
						continue
					}

					speedFeatures = append(speedFeatures, fmt.Sprintf(speedFeaturePattern, speed))
				}
			}
		}
	}

	return
}
