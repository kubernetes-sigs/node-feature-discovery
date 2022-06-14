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

package source_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	source "sigs.k8s.io/node-feature-discovery/source"

	// Register all source packages
	_ "sigs.k8s.io/node-feature-discovery/source/cpu"
	_ "sigs.k8s.io/node-feature-discovery/source/custom"
	_ "sigs.k8s.io/node-feature-discovery/source/fake"
	_ "sigs.k8s.io/node-feature-discovery/source/kernel"
	_ "sigs.k8s.io/node-feature-discovery/source/local"
	_ "sigs.k8s.io/node-feature-discovery/source/memory"
	_ "sigs.k8s.io/node-feature-discovery/source/network"
	_ "sigs.k8s.io/node-feature-discovery/source/pci"
	_ "sigs.k8s.io/node-feature-discovery/source/storage"
	_ "sigs.k8s.io/node-feature-discovery/source/system"
	_ "sigs.k8s.io/node-feature-discovery/source/usb"
)

func TestLabelSources(t *testing.T) {
	sources := source.GetAllLabelSources()
	assert.NotZero(t, len(sources))

	for n, s := range sources {
		assert.Equalf(t, n, s.Name(), "testing labelsource %q failed", n)
	}
}

func TestConfigurableSources(t *testing.T) {
	sources := source.GetAllConfigurableSources()
	assert.NotZero(t, len(sources))

	for n, s := range sources {
		assert.Equalf(t, n, s.Name(), "testing ConfigurableSource %q failed", n)

		c := s.NewConfig()
		s.SetConfig(c)
		rc := s.GetConfig()

		assert.Equalf(t, c, rc, "testing ConfigurableSource %q failed", n)
	}
}

func TestFeatureSources(t *testing.T) {
	sources := source.GetAllFeatureSources()
	assert.NotZero(t, len(sources))

	for n, s := range sources {
		msg := fmt.Sprintf("testing FeatureSource %q failed", n)

		assert.Equal(t, n, s.Name(), msg)

		f := s.GetFeatures()
		assert.NotNil(t, f, msg)
		assert.Empty(t, (*f).Flags, msg)
		assert.Empty(t, (*f).Attributes, msg)
		assert.Empty(t, (*f).Instances, msg)
	}
}
