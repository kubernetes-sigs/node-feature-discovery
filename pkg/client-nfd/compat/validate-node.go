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

package compat

import (
	"context"
	"fmt"

	"oras.land/oras-go/v2/registry"
	"sigs.k8s.io/node-feature-discovery/pkg/client-nfd/compat/match"
	"sigs.k8s.io/node-feature-discovery/pkg/client-nfd/compat/parser"
)

type ValidationResult struct {
	IsNodeValid    bool
	LabelsCheck    map[string]string
	ChecksTotal    int
	ChecksSucceded int
}

func ValidateNode(ctx context.Context, ref *registry.Reference) ([]ValidationResult, error) {
	spec, err := FetchSpec(ctx, ref)
	if err != nil {
		return nil, err
	}

	results := []ValidationResult{}
	for _, c := range spec.Compatibilties {
		result := ValidationResult{}
		result.LabelsCheck = make(map[string]string)

		for k, v := range c.Labels {
			entry, err := parser.Parse(k, v)
			if err != nil {
				return nil, err
			}

			if source, ok := match.Sources[entry.Source()]; ok {
				valid, err := source.Check(entry)
				if err != nil {
					return nil, err
				}

				msg := "\033[31mINVALID\033[0m"
				if valid {
					msg = "\033[32mOK\033[0m"
					result.ChecksSucceded++
				}

				result.LabelsCheck[fmt.Sprintf("%s:%v", entry.KeyRaw(), entry.ValueRaw())] = msg
			} else {
				// TODO: Probably just log a warning
				return nil, fmt.Errorf("unsupported source %q", entry.Source())
			}

			result.ChecksTotal++
		}

		result.IsNodeValid = result.ChecksTotal == result.ChecksSucceded
		results = append(results, result)
	}

	return results, nil
}
