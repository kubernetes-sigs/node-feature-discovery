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
	"fmt"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/client-nfd/compat/parser"
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/kernel"
)

func ValidateKernel(imageLabel parser.EntryGetter, nodeLabels source.FeatureLabels, nodeFeatures *nfdv1alpha1.Features) (bool, error) {
	switch f := imageLabel.Feature(); f {
	case kernel.SelinuxFeature, kernel.VersionFeature:
		// SeLinux and Version are advertised as labels
		// thus it's ok to use the default match
		return ValidateDefault(imageLabel, nodeLabels, nodeFeatures)
	case kernel.ConfigFeature:
		// Kernel config is advertised as labels by enabling specific items over allowed list.
		// It makes more sense to look into discovered features than modifying NFD configuration for the image compatibility validation.
		if v, ok := nodeFeatures.Attributes[kernel.ConfigFeature].Elements[imageLabel.Option()]; ok {
			return imageLabel.ValueEqualTo(v)
		}
	case kernel.LoadedModuleFeature:
		if _, ok := nodeFeatures.Flags[kernel.LoadedModuleFeature].Elements[imageLabel.Option()]; ok {
			return true, nil
		}
	default:
		return false, fmt.Errorf("unsupported kernel feature %q", f)
	}

	return false, nil
}
