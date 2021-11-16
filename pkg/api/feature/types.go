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

package feature

//go:generate ./generate.sh

// Features is a collection of all features of the system, arranged by domain.
// +protobuf
type Features map[string]*DomainFeatures

// DomainFeatures is the collection of all discovered features of one domain.
type DomainFeatures struct {
	Keys      map[string]KeyFeatureSet      `protobuf:"bytes,1,rep,name=keys"`
	Values    map[string]ValueFeatureSet    `protobuf:"bytes,2,rep,name=values"`
	Instances map[string]InstanceFeatureSet `protobuf:"bytes,3,rep,name=instances"`
}

// KeyFeatureSet is a set of simple features only containing names without values.
type KeyFeatureSet struct {
	Elements map[string]Nil `protobuf:"bytes,1,rep,name=elements"`
}

// ValueFeatureSet is a set of features having string value.
type ValueFeatureSet struct {
	Elements map[string]string `protobuf:"bytes,1,rep,name=elements"`
}

// InstanceFeatureSet is a set of features each of which is an instance having multiple attributes.
type InstanceFeatureSet struct {
	Elements []InstanceFeature `protobuf:"bytes,1,rep,name=elements"`
}

// InstanceFeature represents one instance of a complex features, e.g. a device.
type InstanceFeature struct {
	Attributes map[string]string `protobuf:"bytes,1,rep,name=attributes"`
}

// Nil is a dummy empty struct for protobuf compatibility
type Nil struct{}
