/*
Copyright 2018 The Kubernetes Authors.

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
	"io/ioutil"
	"log"

	"sigs.k8s.io/node-feature-discovery/source"
)

// Configuration file options
type cpuidConfig struct {
	AttributeBlacklist []string `json:"attributeBlacklist,omitempty"`
	AttributeWhitelist []string `json:"attributeWhitelist,omitempty"`
}

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
				"SSE4.1",
				"SSE4.2",
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

// Implement FeatureSource interface
type Source struct {
	config      *Config
	cpuidFilter *keyFilter
}

func (s Source) Name() string { return "cpu" }

// NewConfig method of the FeatureSource interface
func (s *Source) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the FeatureSource interface
func (s *Source) GetConfig() source.Config { return s.config }

// SetConfig method of the FeatureSource interface
func (s *Source) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
		s.initCpuidFilter()
	default:
		log.Printf("PANIC: invalid config type: %T", conf)
	}
}

func (s *Source) Discover() (source.Features, error) {
	features := source.Features{}

	// Check if hyper-threading seems to be enabled
	found, err := haveThreadSiblings()
	if err != nil {
		log.Printf("ERROR: failed to detect hyper-threading: %v", err)
	} else if found {
		features["hardware_multithreading"] = true
	}

	// Check SST-BF
	found, err = discoverSSTBF()
	if err != nil {
		log.Printf("ERROR: failed to detect SST-BF: %v", err)
	} else if found {
		features["power.sst_bf.enabled"] = true
	}

	// Detect CPUID
	cpuidFlags := getCpuidFlags()
	for _, f := range cpuidFlags {
		if s.cpuidFilter.unmask(f) {
			features["cpuid."+f] = true
		}
	}

	// Detect pstate features
	pstate, err := detectPstate()
	if err != nil {
		log.Printf("ERROR: %v", err)
	} else {
		for k, v := range pstate {
			features["pstate."+k] = v
		}
	}

	// Detect RDT features
	rdt := discoverRDT()
	for _, f := range rdt {
		features["rdt."+f] = true
	}

	return features, nil
}

// Check if any (online) CPUs have thread siblings
func haveThreadSiblings() (bool, error) {

	files, err := ioutil.ReadDir(source.SysfsDir.Path("bus/cpu/devices"))
	if err != nil {
		return false, err
	}

	for _, file := range files {
		// Try to read siblings from topology
		siblings, err := ioutil.ReadFile(source.SysfsDir.Path("bus/cpu/devices", file.Name(), "topology/thread_siblings_list"))
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

func (s *Source) initCpuidFilter() {
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
