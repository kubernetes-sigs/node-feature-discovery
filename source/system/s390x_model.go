/*
Copyright 2022 The Kubernetes Authors.

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

var s390xModelMap = map[string][]string{
	"z9":   {"2094", "2096"},
	"z10":  {"2097", "2098"},
	"z196": {"2817"},
	"z114": {"2818"},
	"z12":  {"2827", "2828"},
	"z13":  {"2964", "2965"},
	"z14":  {"3906", "3907"},
	"z15":  {"8561", "8562"},
	"z16":  {"3931"},
}

// S390xModels ist the main struct for querying IBM z Systems hardware models
type S390xModels struct {
	HwMap map[string][]string
}

// NewS390xModels creates a new instance of S390xModels
func NewS390xModels() S390xModels {
	s390xmodels := S390xModels{}
	s390xmodels.HwMap = s390xModelMap
	return s390xmodels
}

// LookupModel looks for the IBM z Systems hardware model matching the given machine type
func (s S390xModels) LookupModel(machineType string) (model string) {
	found := false
	for k, v := range s.HwMap {
		if s.stringInSlice(machineType, v) {
			model = k
			found = true
			break
		}
	}
	if !found {
		model = "unknown"
	}
	return
}

func (s S390xModels) stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
