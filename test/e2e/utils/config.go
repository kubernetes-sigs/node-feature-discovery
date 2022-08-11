/*
Copyright 2018-2022 The Kubernetes Authors.

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

package utils

import (
	"flag"
	"os"
	"regexp"

	e2elog "k8s.io/kubernetes/test/e2e/framework/log"

	"sigs.k8s.io/yaml"
)

const (
	DefaultConfigPath             = "/var/lib/kubelet/config.yaml"
	DefaultPodResourcesSocketPath = "/var/lib/kubelet/pod-resources/kubelet.sock"
)

var (
	e2eConfigFile = flag.String("nfd.e2e-config", "", "Configuration parameters for end-to-end tests")

	config *E2EConfig
)

type KubeletConfig struct {
	ConfigPath             string
	PodResourcesSocketPath string
}

type E2EConfig struct {
	DefaultFeatures *struct {
		LabelWhitelist      lookupMap
		AnnotationWhitelist lookupMap
		Nodes               []NodeConfig
	}

	Kubelet *KubeletConfig
}

// GetKubeletConfig returns a KubeletConfig object with default values, possibly overridden by user settings.
func (conf *E2EConfig) GetKubeletConfig() KubeletConfig {
	kcfg := KubeletConfig{
		ConfigPath:             DefaultConfigPath,
		PodResourcesSocketPath: DefaultPodResourcesSocketPath,
	}
	if conf.Kubelet == nil {
		return kcfg
	}
	if conf.Kubelet.ConfigPath != "" {
		kcfg.ConfigPath = conf.Kubelet.ConfigPath
	}
	if conf.Kubelet.PodResourcesSocketPath != "" {
		kcfg.PodResourcesSocketPath = conf.Kubelet.PodResourcesSocketPath
	}
	return kcfg
}

type NodeConfig struct {
	Name                     string
	NodeNameRegexp           string
	ExpectedLabelValues      map[string]string
	ExpectedLabelKeys        lookupMap
	ExpectedAnnotationValues map[string]string
	ExpectedAnnotationKeys   lookupMap

	nameRe *regexp.Regexp
}

type lookupMap map[string]struct{}

func (l *lookupMap) UnmarshalJSON(data []byte) error {
	*l = lookupMap{}

	var slice []string
	if err := yaml.Unmarshal(data, &slice); err != nil {
		return err
	}

	for _, k := range slice {
		(*l)[k] = struct{}{}
	}
	return nil
}

func GetConfig() (*E2EConfig, error) {
	// Read and parse only once
	if config != nil || *e2eConfigFile == "" {
		return config, nil
	}

	data, err := os.ReadFile(*e2eConfigFile)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Pre-compile node name matching regexps
	for i, nodeConf := range config.DefaultFeatures.Nodes {
		config.DefaultFeatures.Nodes[i].nameRe, err = regexp.Compile(nodeConf.NodeNameRegexp)
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}

func FindNodeConfig(cfg *E2EConfig, nodeName string) *NodeConfig {
	var nodeConf *NodeConfig
	for _, conf := range cfg.DefaultFeatures.Nodes {
		if conf.nameRe.MatchString(nodeName) {
			e2elog.Logf("node %q matches rule %q (regexp %q)", nodeName, conf.Name, conf.nameRe)
			nodeConf = &conf
			break
		}
	}
	return nodeConf
}
