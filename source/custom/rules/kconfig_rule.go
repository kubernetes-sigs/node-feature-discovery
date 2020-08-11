/*
Copyright 2020 The Kubernetes Authors.

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

package rules

import (
	"fmt"
	"sigs.k8s.io/node-feature-discovery/source/internal/kernelutils"
)

// KconfigRule implements Rule
type KconfigRule []string

var kConfigs map[string]struct{}

func (kconfigs *KconfigRule) Match() (bool, error) {
	for _, f := range *kconfigs {
		if _, ok := kConfigs[f]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func init() {
	kConfigs = make(map[string]struct{})

	kconfig, err := kernelutils.ParseKconfig("")
	if err == nil {
		for k, v := range kconfig {
			if v != "true" {
				kConfigs[fmt.Sprintf("%s=%s", k, v)] = struct{}{}
			} else {
				kConfigs[k] = struct{}{}
			}
		}
	}
}
