/*
Copyright 2020-2021 The Kubernetes Authors.

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
	"encoding/json"
	"strings"

	"sigs.k8s.io/node-feature-discovery/source/internal/kernelutils"
)

// KconfigRule implements Rule
type KconfigRule []kconfig

type kconfig struct {
	Name  string
	Value string
}

var kConfigs map[string]string

func (kconfigs *KconfigRule) Match() (bool, error) {
	for _, f := range *kconfigs {
		if v, ok := kConfigs[f.Name]; !ok || f.Value != v {
			return false, nil
		}
	}
	return true, nil
}

func (c *kconfig) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	split := strings.SplitN(raw, "=", 2)
	c.Name = split[0]
	if len(split) == 1 {
		c.Value = "true"
	} else {
		c.Value = split[1]
	}
	return nil
}

func init() {
	kConfigs = make(map[string]string)

	kconfig, err := kernelutils.ParseKconfig("")
	if err == nil {
		for k, v := range kconfig {
			kConfigs[k] = v
		}
	}
}
