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

package v1alpha1

import (
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

// ArtifactType is a type of OCI artifact that contains image compatibility metadata.
const (
	ArtifactType = "application/vnd.nfd.image-compatibility.v1alpha1"
	Version      = "v1alpha1"
)

// Spec represents image compatibility metadata.
type Spec struct {
	// Version of the spec.
	Version string `json:"version"`
	// Compatibilities contains list of compatibility sets.
	Compatibilties []Compatibility `json:"compatibilities"`
}

// Compatibility represents image compatibility metadata
// that describe the image requirements for the host and OS.
type Compatibility struct {
	// Rules represents a list of Node Feature Rules.
	Rules []nfdv1alpha1.GroupRule `json:"rules"`
	// Weight indicates the priority of the compatibility set.
	Weight int `json:"weight,omitempty"`
	// Tag enables grouping or distinguishing between compatibility sets.
	Tag string `json:"tag,omitempty"`
	// Description of the compatibility set.
	Description string `json:"description,omitempty"`
}
