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
	"encoding/json"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"

	compatv1alpha1 "sigs.k8s.io/node-feature-discovery/api/image-compatibility/v1alpha1"
)

func FetchSpec(ctx context.Context, ref *registry.Reference) (*compatv1alpha1.Spec, error) {
	repo, err := remote.NewRepository(ref.String())
	if err != nil {
		return nil, err
	}
	// TODO: remove - just experimental
	repo.PlainHTTP = true

	targetDesc, err := oras.Resolve(ctx, repo, ref.Reference, oras.DefaultResolveOptions)
	if err != nil {
		return nil, err
	}

	descs, err := registry.Referrers(ctx, repo, targetDesc, compatv1alpha1.ArtifactType)
	if err != nil {
		return nil, nil
	} else if len(descs) < 1 {
		return nil, fmt.Errorf("compatibility artifact not found")
	}
	artifactDesc := descs[len(descs)-1]

	_, content, err := oras.FetchBytes(ctx, repo.Manifests(), artifactDesc.Digest.String(), oras.DefaultFetchBytesOptions)
	if err != nil {
		return nil, err
	}

	manifest := ocispec.Manifest{}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, err
	}

	// TODO: now it's a lazy check, verify in the future the media types and number of layers
	if len(manifest.Layers) < 1 {
		return nil, fmt.Errorf("compatibility layer not found")
	}
	specDesc := manifest.Layers[0]

	_, specRaw, err := oras.FetchBytes(ctx, repo.Blobs(), specDesc.Digest.String(), oras.DefaultFetchBytesOptions)
	if err != nil {
		return nil, err
	}

	spec := compatv1alpha1.Spec{}
	err = json.Unmarshal(specRaw, &spec)
	if err != nil {
		return nil, err
	}

	return &spec, nil
}
