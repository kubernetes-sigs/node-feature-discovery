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

package security

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

const (
	SecureBootString = "SecureBoot enabled"
)

// Source implements FeatureSource.
type Source struct{}

func (s Source) Name() string { return "security" }

// Returns feature names for SecureBoot and BootGuard if suppported.
func (s Source) Discover() ([]string, error) {
	features := []string{}

	//Discovery of SecureBoot
	mokutilOutBytes, err := exec.Command("mokutil", "--sb-state").Output()
	if err != nil {
		glog.Errorf("Failed to detect support for UEFISecureBoot: %v", err)
	} else {
		mokutilOut := bytes.TrimSpace(mokutilOutBytes)
		glog.Infof("The output after running mokutil --sb-state is %s\n", mokutilOut)
		if strings.Compare(string(mokutilOut), SecureBootString) == 0 {
			features = append(features, "UEFISecureBoot")
		} else {
			glog.Errorf("UEFISecureBoot not enabled on the system\n")
		}
	}

	//Discovery of BootGuard
	out, err := exec.Command("rdmsr", "0x13a", "-f", "0:0").Output()
	if err != nil {
		glog.Errorf("Error encountered while detectng support for BootGuard: %v", err)
	} else {
		glog.Infof("The output after running rdmsr 0x13a -f 0:0 is %s\n", out)
		x, err := strconv.Atoi(string(out[0]))
		if err != nil {
			glog.Errorf("Error converting MSR value to integer: %v\n",err)

		} else {

			if x == 1 {
				features = append(features, "BootGuard")
			} else {
				glog.Errorf("BootGuard not enabled on the system\n")
			}

		}
	}

	return features, nil
}
