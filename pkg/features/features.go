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

package features

import (
	"sigs.k8s.io/node-feature-discovery/pkg/utils/featuregate"
)

const (
	NodeFeatureAPI featuregate.Feature = "NodeFeatureAPI"
)

var (
	NFDMutableFeatureGate featuregate.MutableFeatureGate = featuregate.NewFeatureGate()

	// NFDFeatureGate is a shared global FeatureGate.
	// Top-level commands/options setup that needs to modify this feature gate should use NFDMutableFeatureGate.
	NFDFeatureGate featuregate.FeatureGate = NFDMutableFeatureGate
)

var DefaultNFDFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	NodeFeatureAPI: {Default: true, PreRelease: featuregate.Beta},
}
