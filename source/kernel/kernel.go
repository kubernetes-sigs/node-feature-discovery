/*
Copyright 2018-2021 The Kubernetes Authors.

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

package kernel

import (
	"strconv"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
)

const Name = "kernel"

const (
	ConfigFeature       = "config"
	LoadedModuleFeature = "loadedmodule"
	SelinuxFeature      = "selinux"
	VersionFeature      = "version"
)

// Configuration file options
type Config struct {
	KconfigFile string
	ConfigOpts  []string `json:"configOpts,omitempty"`
}

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *Config {
	return &Config{
		KconfigFile: "",
		ConfigOpts: []string{
			"NO_HZ",
			"NO_HZ_IDLE",
			"NO_HZ_FULL",
			"PREEMPT",
		},
	}
}

// kernelSource implements the FeatureSource, LabelSource and ConfigurableSource interfaces.
type kernelSource struct {
	config   *Config
	features *feature.DomainFeatures
}

// Singleton source instance
var (
	src                           = kernelSource{config: newDefaultConfig()}
	_   source.FeatureSource      = &src
	_   source.LabelSource        = &src
	_   source.ConfigurableSource = &src
)

func (s *kernelSource) Name() string { return Name }

// NewConfig method of the LabelSource interface
func (s *kernelSource) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the LabelSource interface
func (s *kernelSource) GetConfig() source.Config { return s.config }

// SetConfig method of the LabelSource interface
func (s *kernelSource) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
	default:
		klog.Fatalf("invalid config type: %T", conf)
	}
}

// Priority method of the LabelSource interface
func (s *kernelSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *kernelSource) GetLabels() (source.FeatureLabels, error) {
	labels := source.FeatureLabels{}
	features := s.GetFeatures()

	for k, v := range features.Values[VersionFeature].Elements {
		labels[VersionFeature+"."+k] = v
	}

	// Check flags
	for _, opt := range s.config.ConfigOpts {
		if val, ok := features.Values[ConfigFeature].Elements[opt]; ok {
			labels[ConfigFeature+"."+opt] = val
		}
	}

	for k, v := range features.Values[SelinuxFeature].Elements {
		labels[SelinuxFeature+"."+k] = v
	}

	return labels, nil
}

// Discover method of the FeatureSource interface
func (s *kernelSource) Discover() error {
	s.features = feature.NewDomainFeatures()

	// Read kernel version
	if version, err := parseVersion(); err != nil {
		klog.Errorf("failed to get kernel version: %s", err)
	} else {
		s.features.Values[VersionFeature] = feature.NewValueFeatures(version)
	}

	// Read kconfig
	if kconfig, err := parseKconfig(s.config.KconfigFile); err != nil {
		klog.Errorf("failed to read kconfig: %s", err)
	} else {
		s.features.Values[ConfigFeature] = feature.NewValueFeatures(kconfig)
	}

	if kmods, err := getLoadedModules(); err != nil {
		klog.Errorf("failed to get loaded kernel modules: %v", err)
	} else {
		s.features.Keys[LoadedModuleFeature] = feature.NewKeyFeatures(kmods...)
	}

	if selinux, err := SelinuxEnabled(); err != nil {
		klog.Warning(err)
	} else {
		s.features.Values[SelinuxFeature] = feature.NewValueFeatures(nil)
		s.features.Values[SelinuxFeature].Elements["enabled"] = strconv.FormatBool(selinux)
	}

	utils.KlogDump(3, "discovered kernel features:", "  ", s.features)

	return nil
}

func (s *kernelSource) GetFeatures() *feature.DomainFeatures {
	if s.features == nil {
		s.features = feature.NewDomainFeatures()
	}
	return s.features
}

func init() {
	source.Register(&src)
}
