//go:build !(linux && s390x)

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

package crypto

import (
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

// detectCexCards is a stub for non-s390x platforms.
// CEX cards are only available on s390x architecture.
func detectCexCards() ([]nfdv1alpha1.InstanceFeature, error) {
	return nil, nil
}
