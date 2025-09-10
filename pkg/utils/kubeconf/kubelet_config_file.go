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
	"context"
	"fmt"

	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	kubeletconfigscheme "k8s.io/kubernetes/pkg/kubelet/apis/config/scheme"
	"k8s.io/kubernetes/pkg/kubelet/kubeletconfig/configfiles"
	utilfs "k8s.io/kubernetes/pkg/util/filesystem"
)

// GetKubeletConfigFromLocalFile returns KubeletConfiguration loaded from the node local config
// based on https://github.com/kubernetes/kubernetes/blob/master/cmd/kubelet/app/server.go#L337
// it fills empty fields with default values
func GetKubeletConfigFromLocalFile(kubeletConfigPath string) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
	const errFmt = "failed to load Kubelet config file %s, error %w"

	loader, err := configfiles.NewFsLoader(&utilfs.DefaultFs{}, kubeletConfigPath)
	if err != nil {
		return nil, fmt.Errorf(errFmt, kubeletConfigPath, err)
	}

	kc, err := loader.Load(context.Background())
	if err != nil {
		return nil, fmt.Errorf(errFmt, kubeletConfigPath, err)
	}

	scheme, _, err := kubeletconfigscheme.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}

	kubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{}
	err = scheme.Convert(kc, kubeletConfig, nil)
	if err != nil {
		return nil, err
	}

	return kubeletConfig, nil
}
