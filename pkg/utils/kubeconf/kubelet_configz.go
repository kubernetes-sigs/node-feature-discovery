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

package kubeconf

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
)

// GetKubeletConfiguration returns the kubelet configuration.
func GetKubeletConfiguration(restConfig *rest.Config) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	var timeout time.Duration
	// This hack because /configz reports the following structure:
	// {"kubeletconfig": {the JSON representation of kubeletconfigv1beta1.KubeletConfiguration}}
	type configzWrapper struct {
		ComponentConfig kubeletconfigv1beta1.KubeletConfiguration `json:"kubeletconfig"`
	}
	bytes, err := discoveryClient.RESTClient().
		Get().
		Timeout(timeout).
		Do(context.TODO()).
		Raw()
	if err != nil {
		return nil, err
	}

	configz := configzWrapper{}
	if err = json.Unmarshal(bytes, &configz); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json for kubelet config: %w", err)
	}

	return &configz.ComponentConfig, nil
}

// InsecureConfig returns a kubelet API config object which uses the token path.
func InsecureConfig(host, tokenFile string) (*rest.Config, error) {
	if tokenFile == "" {
		return nil, fmt.Errorf("api auth token file must be defined")
	}
	if len(host) == 0 {
		return nil, fmt.Errorf("kubelet host must be defined")
	}

	token, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, err
	}

	tlsClientConfig := rest.TLSClientConfig{Insecure: true}

	return &rest.Config{
		Host:            host,
		TLSClientConfig: tlsClientConfig,
		BearerToken:     string(token),
		BearerTokenFile: tokenFile,
	}, nil
}
