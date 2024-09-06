/*
Copyright 2024 The Kubernetes Authors.

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

package match

import (
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/client-nfd/compat/parser"
	"sigs.k8s.io/node-feature-discovery/source"

	// Register sources for validation
	"sigs.k8s.io/node-feature-discovery/source/cpu"
	"sigs.k8s.io/node-feature-discovery/source/kernel"
	"sigs.k8s.io/node-feature-discovery/source/local"
	"sigs.k8s.io/node-feature-discovery/source/memory"
	"sigs.k8s.io/node-feature-discovery/source/network"
	"sigs.k8s.io/node-feature-discovery/source/pci"
	"sigs.k8s.io/node-feature-discovery/source/storage"
	"sigs.k8s.io/node-feature-discovery/source/system"
	"sigs.k8s.io/node-feature-discovery/source/usb"
)

var Sources map[string]Matcher = make(map[string]Matcher)

type ValidateFunc func(imageLabel parser.EntryGetter, nodeLabels source.FeatureLabels, nodeFeatures *nfdv1alpha1.Features) (bool, error)

type Matcher interface {
	Init() error
	Check(parser.EntryGetter) (bool, error)
}

func init() {
	// Register sources with default match function
	Sources[cpu.Name] = NewGenericMatcher(cpu.Name, ValidateDefault)
	// TODO: add support for custom features
	// Sources[custom.Name] = NewGenericMatcher(custom.Name, ValidateDefault)
	Sources[local.Name] = NewGenericMatcher(local.Name, ValidateDefault)
	Sources[memory.Name] = NewGenericMatcher(memory.Name, ValidateDefault)
	Sources[network.Name] = NewGenericMatcher(network.Name, ValidateDefault)
	Sources[pci.Name] = NewGenericMatcher(pci.Name, ValidateDefault)
	Sources[storage.Name] = NewGenericMatcher(storage.Name, ValidateDefault)
	Sources[system.Name] = NewGenericMatcher(system.Name, ValidateDefault)
	Sources[usb.Name] = NewGenericMatcher(usb.Name, ValidateDefault)

	// Register kernel source with custom kernel match function.
	// Kernel modules and config are not reported as labels, thus
	// it's necessary to have lookup into raw data and have a custom match function
	Sources[kernel.Name] = NewGenericMatcher(kernel.Name, ValidateKernel)
}
