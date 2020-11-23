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

package kernel

import (
	"log"
	"regexp"
	"strings"

	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/internal/kernelutils"
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

// Implement FeatureSource interface
type Source struct {
	config *Config
}

func (s *Source) Name() string { return "kernel" }

// NewConfig method of the FeatureSource interface
func (s *Source) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the FeatureSource interface
func (s *Source) GetConfig() source.Config { return s.config }

// SetConfig method of the FeatureSource interface
func (s *Source) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
	default:
		log.Printf("PANIC: invalid config type: %T", conf)
	}
}

func (s *Source) Discover() (source.Features, error) {
	features := source.Features{}

	// Read kernel version
	version, err := parseVersion()
	if err != nil {
		log.Printf("ERROR: Failed to get kernel version: %s", err)
	} else {
		for key := range version {
			features["version."+key] = version[key]
		}
	}

	// Read kconfig
	kconfig, err := kernelutils.ParseKconfig(s.config.KconfigFile)
	if err != nil {
		log.Printf("ERROR: Failed to read kconfig: %s", err)
	}

	// Check flags
	for _, opt := range s.config.ConfigOpts {
		if val, ok := kconfig[opt]; ok {
			features["config."+opt] = val
		}
	}

	selinux, err := SelinuxEnabled()
	if err != nil {
		log.Print(err)
	} else if selinux {
		features["selinux.enabled"] = true
	}

	return features, nil
}

// Read and parse kernel version
func parseVersion() (map[string]string, error) {
	version := map[string]string{}

	full, err := kernelutils.GetKernelVersion()
	if err != nil {
		return nil, err
	}

	// Replace forbidden symbols
	fullRegex := regexp.MustCompile("[^-A-Za-z0-9_.]")
	full = fullRegex.ReplaceAllString(full, "_")
	// Label values must start and end with an alphanumeric
	full = strings.Trim(full, "-_.")

	version["full"] = full

	// Regexp for parsing version components
	re := regexp.MustCompile(`^(?P<major>\d+)(\.(?P<minor>\d+))?(\.(?P<revision>\d+))?(-.*)?$`)
	if m := re.FindStringSubmatch(full); m != nil {
		for i, name := range re.SubexpNames() {
			if i != 0 && name != "" {
				version[name] = m[i]
			}
		}
	}

	return version, nil
}
