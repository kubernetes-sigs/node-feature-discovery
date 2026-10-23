/*
Copyright 2026 The Kubernetes Authors.

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

package v1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAddToScheme(t *testing.T) {
	s := runtime.NewScheme()
	if err := AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	for _, kind := range []string{"NodeFeatureDiscovery", "NodeFeatureDiscoveryList", "ListOptions"} {
		gvk := schema.GroupVersionKind{Group: "nfd.kubernetes.io", Version: "v1", Kind: kind}
		if !s.Recognizes(gvk) {
			t.Errorf("scheme does not recognize %s", gvk)
		}
	}

	gvks, _, err := s.ObjectKinds(&NodeFeatureDiscovery{})
	if err != nil {
		t.Fatalf("ObjectKinds: %v", err)
	}
	if len(gvks) != 1 || gvks[0] != (schema.GroupVersionKind{Group: "nfd.kubernetes.io", Version: "v1", Kind: "NodeFeatureDiscovery"}) {
		t.Errorf("NodeFeatureDiscovery kinds = %v, want exactly nfd.kubernetes.io/v1, Kind=NodeFeatureDiscovery", gvks)
	}
}
