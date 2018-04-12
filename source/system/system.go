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

package system

import (
	"log"

	"sigs.k8s.io/node-feature-discovery/source"
)

// Source implements FeatureSource.
type Source struct{}

// Name returns an identifier string for this feature source.
func (s Source) Name() string { return "system" }

// Discover returns feature names for system configuration: irqbalance, swap, ksm, memory compaction, iommu...
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}
	active, err := SystemctlStatusIsActive("irqbalance")

	if err != nil {
		log.Printf("ERROR: Failed to check irqbalance: %s", err)
	} else {
		features["systemd.irqbalance"] = active
	}

	return features, nil
}
