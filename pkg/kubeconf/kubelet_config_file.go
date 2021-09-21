/*
Copyright 2021 The Kubernetes Authors.

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

package kubeconf

import (
	"io/ioutil"

	"github.com/ghodss/yaml"

	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
)

// GetKubeletConfigFromLocalFile returns KubeletConfiguration loaded from the node local config
func GetKubeletConfigFromLocalFile(kubeletConfigPath string) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
	kubeletBytes, err := ioutil.ReadFile(kubeletConfigPath)
	if err != nil {
		return nil, err
	}

	kubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{}
	if err := yaml.Unmarshal(kubeletBytes, kubeletConfig); err != nil {
		return nil, err
	}
	return kubeletConfig, nil
}
