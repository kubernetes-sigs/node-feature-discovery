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
