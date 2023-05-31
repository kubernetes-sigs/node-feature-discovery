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

package cpu

import (
	"fmt"
	"os"
	"strconv"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	"github.com/klauspost/cpuid/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Name of this feature source
const Name = "cpu"

const (
	CpuidFeature       = "cpuid"
	Cpumodel           = "model"
	CstateFeature      = "cstate"
	PstateFeature      = "pstate"
	RdtFeature         = "rdt"
	SeFeature          = "se" // DEPRECATED in v0.12: will be removed in the future
	SecurityFeature    = "security"
	SgxFeature         = "sgx" // DEPRECATED in v0.12: will be removed in the future
	SstFeature         = "sst"
	TopologyFeature    = "topology"
	CoprocessorFeature = "coprocessor"
)

// Configuration file options
type cpuidConfig struct {
	AttributeBlacklist []string `json:"attributeBlacklist,omitempty"`
	AttributeWhitelist []string `json:"attributeWhitelist,omitempty"`
}

// Config holds configuration for the cpu source.
type Config struct {
	Cpuid cpuidConfig `json:"cpuid,omitempty"`
}

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *Config {
	return &Config{
		cpuidConfig{
			AttributeBlacklist: []string{
				"BMI1",
				"BMI2",
				"CLMUL",
				"CMOV",
				"CX16",
				"ERMS",
				"F16C",
				"HTT",
				"LZCNT",
				"MMX",
				"MMXEXT",
				"NX",
				"POPCNT",
				"RDRAND",
				"RDSEED",
				"RDTSCP",
				"SGX",
				"SGXLC",
				"SSE",
				"SSE2",
				"SSE3",
				"SSE4",
				"SSE42",
				"SSSE3",
			},
			AttributeWhitelist: []string{},
		},
	}
}

// Filter for cpuid labels
type keyFilter struct {
	keys      map[string]struct{}
	whitelist bool
}

// cpuSource implements the FeatureSource, LabelSource and ConfigurableSource interfaces.
type cpuSource struct {
	config      *Config
	cpuidFilter *keyFilter
	features    *nfdv1alpha1.Features
}

// Singleton source instance
var (
	src                           = cpuSource{config: newDefaultConfig(), cpuidFilter: &keyFilter{}}
	_   source.FeatureSource      = &src
	_   source.LabelSource        = &src
	_   source.ConfigurableSource = &src
)

func (s *cpuSource) Name() string { return Name }

// NewConfig method of the LabelSource interface
func (s *cpuSource) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the LabelSource interface
func (s *cpuSource) GetConfig() source.Config { return s.config }

// SetConfig method of the LabelSource interface
func (s *cpuSource) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
		s.initCpuidFilter()
	default:
		panic(fmt.Sprintf("invalid config type: %T", conf))
	}
}

// Priority method of the LabelSource interface
func (s *cpuSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *cpuSource) GetLabels() (source.FeatureLabels, error) {
	labels := source.FeatureLabels{}
	features := s.GetFeatures()

	// CPUID
	for f := range features.Flags[CpuidFeature].Elements {
		if s.cpuidFilter.unmask(f) {
			labels["cpuid."+f] = true
		}
	}

	// CPU model
	for k, v := range features.Attributes[Cpumodel].Elements {
		labels["model."+k] = v
	}

	// Cstate
	for k, v := range features.Attributes[CstateFeature].Elements {
		labels["cstate."+k] = v
	}

	// Pstate
	for k, v := range features.Attributes[PstateFeature].Elements {
		labels["pstate."+k] = v
	}

	// RDT
	for k, v := range features.Attributes[RdtFeature].Elements {
		if k == "RDTL3CA_NUM_CLOSID" {
			continue
		}

		labels["rdt."+k] = v
	}

	// Security
	// skipLabel lists features that will not have labels created but are only made available for
	// NodeFeatureRules (e.g. to be published via extended resources instead)
	skipLabel := sets.NewString(
		"tdx.total_keys",
		"sgx.epc",
		"sev.encrypted_state_ids",
		"sev.asids")
	for k, v := range features.Attributes[SecurityFeature].Elements {
		if !skipLabel.Has(k) {
			labels["security."+k] = v
		}
	}

	// SGX
	for k, v := range features.Attributes[SgxFeature].Elements {
		labels["sgx."+k] = v
	}

	// Secure Execution
	for k, v := range features.Attributes[SeFeature].Elements {
		labels["se."+k] = v
	}

	// SST
	for k, v := range features.Attributes[SstFeature].Elements {
		labels["power.sst_"+k] = v
	}

	// Hyperthreading
	if v, ok := features.Attributes[TopologyFeature].Elements["hardware_multithreading"]; ok {
		labels["hardware_multithreading"] = v
	}

	// NX
	if v, ok := features.Attributes[CoprocessorFeature].Elements["nx_gzip"]; ok {
		labels["coprocessor.nx_gzip"] = v
	}

	return labels, nil
}

// Discover method of the FeatureSource Interface
func (s *cpuSource) Discover() error {
	s.features = nfdv1alpha1.NewFeatures()

	// Detect CPUID
	s.features.Flags[CpuidFeature] = nfdv1alpha1.NewFlagFeatures(getCpuidFlags()...)

	// Detect CPU model
	s.features.Attributes[Cpumodel] = nfdv1alpha1.NewAttributeFeatures(getCPUModel())

	// Detect cstate configuration
	cstate, err := detectCstate()
	if err != nil {
		klog.ErrorS(err, "failed to detect cstate")
	} else {
		s.features.Attributes[CstateFeature] = nfdv1alpha1.NewAttributeFeatures(cstate)
	}

	// Detect pstate features
	pstate, err := detectPstate()
	if err != nil {
		klog.ErrorS(err, "failed to detect pstate")
	}
	s.features.Attributes[PstateFeature] = nfdv1alpha1.NewAttributeFeatures(pstate)

	// Detect RDT features
	s.features.Attributes[RdtFeature] = nfdv1alpha1.NewAttributeFeatures(discoverRDT())

	// Detect available guest protection(SGX,TDX,SEV) features
	s.features.Attributes[SecurityFeature] = nfdv1alpha1.NewAttributeFeatures(discoverSecurity())

	// Detect SGX features
	//
	// DEPRECATED in v0.12: will be removed in the future
	if val, ok := s.features.Attributes[SecurityFeature].Elements["sgx.enabled"]; ok {
		s.features.Attributes[SgxFeature] = nfdv1alpha1.NewAttributeFeatures(map[string]string{"enabled": val})
	}

	// Detect Secure Execution features
	//
	// DEPRECATED in v0.12: will be removed in the future
	if val, ok := s.features.Attributes[SecurityFeature].Elements["se.enabled"]; ok {
		s.features.Attributes[SeFeature] = nfdv1alpha1.NewAttributeFeatures(map[string]string{"enabled": val})
	}

	// Detect SST features
	s.features.Attributes[SstFeature] = nfdv1alpha1.NewAttributeFeatures(discoverSST())

	// Detect hyper-threading
	s.features.Attributes[TopologyFeature] = nfdv1alpha1.NewAttributeFeatures(discoverTopology())

	// Detect Coprocessor features
	s.features.Attributes[CoprocessorFeature] = nfdv1alpha1.NewAttributeFeatures(discoverCoprocessor())

	klog.V(3).InfoS("discovered features", "featureSource", s.Name(), "features", utils.DelayedDumper(s.features))

	return nil
}

// GetFeatures method of the FeatureSource Interface
func (s *cpuSource) GetFeatures() *nfdv1alpha1.Features {
	if s.features == nil {
		s.features = nfdv1alpha1.NewFeatures()
	}
	return s.features
}

func getCPUModel() map[string]string {
	cpuModelInfo := make(map[string]string)
	cpuModelInfo["vendor_id"] = cpuid.CPU.VendorID.String()
	cpuModelInfo["family"] = strconv.Itoa(cpuid.CPU.Family)
	cpuModelInfo["id"] = strconv.Itoa(cpuid.CPU.Model)

	return cpuModelInfo
}

func discoverTopology() map[string]string {
	features := make(map[string]string)

	if ht, err := haveThreadSiblings(); err != nil {
		klog.ErrorS(err, "failed to detect hyper-threading")
	} else {
		features["hardware_multithreading"] = strconv.FormatBool(ht)
	}

	return features
}

// Check if any (online) CPUs have thread siblings
func haveThreadSiblings() (bool, error) {

	files, err := os.ReadDir(hostpath.SysfsDir.Path("bus/cpu/devices"))
	if err != nil {
		return false, err
	}

	for _, file := range files {
		// Try to read siblings from topology
		siblings, err := os.ReadFile(hostpath.SysfsDir.Path("bus/cpu/devices", file.Name(), "topology/thread_siblings_list"))
		if err != nil {
			return false, err
		}
		for _, char := range siblings {
			// If list separator found, we determine that there are multiple siblings
			if char == ',' || char == '-' {
				return true, nil
			}
		}
	}
	// No siblings were found
	return false, nil
}

func (s *cpuSource) initCpuidFilter() {
	newFilter := keyFilter{keys: map[string]struct{}{}}
	if len(s.config.Cpuid.AttributeWhitelist) > 0 {
		for _, k := range s.config.Cpuid.AttributeWhitelist {
			newFilter.keys[k] = struct{}{}
		}
		newFilter.whitelist = true
	} else {
		for _, k := range s.config.Cpuid.AttributeBlacklist {
			newFilter.keys[k] = struct{}{}
		}
		newFilter.whitelist = false
	}
	s.cpuidFilter = &newFilter
}

func (f keyFilter) unmask(k string) bool {
	if f.whitelist {
		if _, ok := f.keys[k]; ok {
			return true
		}
	} else {
		if _, ok := f.keys[k]; !ok {
			return true
		}
	}
	return false
}

func init() {
	source.Register(&src)
}
