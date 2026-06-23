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
	"net/url"

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

func GetKubeletConfigFunc(uri, apiAuthTokenFile string) (func() (*kubeletconfigv1beta1.KubeletConfiguration, error), error) {
	u, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse -kubelet-config-uri: %w", err)
	}

	// init kubelet API client
	var klConfig *kubeletconfigv1beta1.KubeletConfiguration
	switch u.Scheme {
	case "file":
		return func() (*kubeletconfigv1beta1.KubeletConfiguration, error) {
			klConfig, err = GetKubeletConfigFromLocalFile(u.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to read kubelet config: %w", err)
			}
			return klConfig, err
		}, nil
	case "https":
		restConfig, err := InsecureConfig(u.String(), apiAuthTokenFile)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize rest config for kubelet config uri: %w", err)
		}

		return func() (*kubeletconfigv1beta1.KubeletConfiguration, error) {
			klConfig, err = GetKubeletConfiguration(restConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to get kubelet config from configz endpoint: %w", err)
			}
			return klConfig, nil
		}, nil
	}

	return nil, fmt.Errorf("unsupported URI scheme: %v", u.Scheme)
}
