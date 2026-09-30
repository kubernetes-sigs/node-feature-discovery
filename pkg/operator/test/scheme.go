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

package test

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
)

func TestScheme() (*runtime.Scheme, error) {
	s := runtime.NewScheme()

	funcs := []func(s *runtime.Scheme) error{
		scheme.AddToScheme,
		nfdv1.AddToScheme,
	}

	for _, f := range funcs {
		if err := f(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}
