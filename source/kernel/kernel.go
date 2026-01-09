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
	"fmt"
	"strconv"

	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Name of this feature source
const Name = "kernel"

const (
	ConfigFeature        = "config"
	LoadedModuleFeature  = "loadedmodule"
	SelinuxFeature       = "selinux"
	VersionFeature       = "version"
	EnabledModuleFeature = "enabledmodule"
	KvmFeature           = "kvm"
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
	features *nfdv1alpha1.Features
	// legacyKconfig contains mangled kconfig values used for
	// kernel.config-<flag> labels and legacy kConfig custom rules.
	legacyKconfig map[string]string
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
		panic(fmt.Sprintf("invalid config type: %T", conf))
	}
}

// Priority method of the LabelSource interface
func (s *kernelSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *kernelSource) GetLabels() (source.FeatureLabels, error) {
	labels := source.FeatureLabels{}
	features := s.GetFeatures()

	for k, v := range features.Attributes[VersionFeature].Elements {
		labels[VersionFeature+"."+k] = v
	}

	for _, opt := range s.config.ConfigOpts {
		if val, ok := s.legacyKconfig[opt]; ok {
			labels[ConfigFeature+"."+opt] = val
		}
	}

	if enabled, ok := features.Attributes[SelinuxFeature].Elements["enabled"]; ok && enabled == "true" {
		labels["selinux.enabled"] = "true"
	}

	return labels, nil
}

// Discover method of the FeatureSource interface
func (s *kernelSource) Discover() error {
	s.features = nfdv1alpha1.NewFeatures()

	// Read kernel version
	if version, err := discoverVersion(); err != nil {
		klog.ErrorS(err, "failed to get kernel version")
	} else {
		s.features.Attributes[VersionFeature] = nfdv1alpha1.NewAttributeFeatures(version)
	}

	// Read kconfig
	if realKconfig, legacyKconfig, err := parseKconfig(s.config.KconfigFile); err != nil {
		s.legacyKconfig = nil
		klog.ErrorS(err, "failed to read kconfig")
	} else {
		s.features.Attributes[ConfigFeature] = nfdv1alpha1.NewAttributeFeatures(realKconfig)
		s.legacyKconfig = legacyKconfig
	}

	var enabledModules []string
	if kmods, err := getLoadedModules(); err != nil {
		klog.ErrorS(err, "failed to get loaded kernel modules")
	} else {
		enabledModules = append(enabledModules, kmods...)
		s.features.Flags[LoadedModuleFeature] = nfdv1alpha1.NewFlagFeatures(kmods...)
	}

	if builtinMods, err := getBuiltinModules(); err != nil {
		klog.ErrorS(err, "failed to get builtin kernel modules")
	} else {
		enabledModules = append(enabledModules, builtinMods...)
		s.features.Flags[EnabledModuleFeature] = nfdv1alpha1.NewFlagFeatures(enabledModules...)
	}

	if selinux, err := SelinuxEnabled(); err != nil {
		klog.ErrorS(err, "failed to detect selinux status")
	} else {
		s.features.Attributes[SelinuxFeature] = nfdv1alpha1.NewAttributeFeatures(nil)
		s.features.Attributes[SelinuxFeature].Elements["enabled"] = strconv.FormatBool(selinux)
	}

	if kvm, err := KvmEnabled(); err != nil {
		klog.ErrorS(err, "failed to detect kvm status")
	} else {
		s.features.Attributes[KvmFeature] = nfdv1alpha1.NewAttributeFeatures(nil)
		s.features.Attributes[KvmFeature].Elements["enabled"] = strconv.FormatBool(kvm)
	}

	klog.V(3).InfoS("discovered features", "featureSource", s.Name(), "features", utils.DelayedDumper(s.features))

	return nil
}

func (s *kernelSource) GetFeatures() *nfdv1alpha1.Features {
	if s.features == nil {
		s.features = nfdv1alpha1.NewFeatures()
	}
	return s.features
}

func GetLegacyKconfig() map[string]string {
	return src.legacyKconfig
}

func init() {
	source.Register(&src)
}
