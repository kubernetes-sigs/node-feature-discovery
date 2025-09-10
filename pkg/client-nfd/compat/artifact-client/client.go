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

//go:generate go tool mockery --name=ArtifactClient --inpackage

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
	"sigs.k8s.io/yaml"

	compatv1alpha1 "sigs.k8s.io/node-feature-discovery/api/image-compatibility/v1alpha1"
)

const (
	ArtifactCreationTimestampKey = "org.opencontainers.image.created"
)

// ArtifactClient interface contain set of functions to manipulate compatibility artfact.
type ArtifactClient interface {
	// FetchCompatibilitySpec downloads the compatibility specifcation associated with the image.
	FetchCompatibilitySpec(ctx context.Context) (*compatv1alpha1.Spec, error)
}

// Args holds command line arguments.
type Args struct {
	PlainHttp bool
}

// Client represents a client that is reposnible for all artifact operations.
type Client struct {
	Args         Args
	RegReference *registry.Reference
	Platform     *ocispec.Platform

	orasClient *auth.Client
}

// New returns a new compatibility spec object.
func New(regReference *registry.Reference, opts ...ArtifactClientOpts) *Client {
	c := &Client{
		RegReference: regReference,
	}
	for _, opt := range opts {
		opt.apply(c)
	}
	return c
}

// FetchCompatibilitySpec pulls the image compatibility specification associated with the image.
func (c *Client) FetchCompatibilitySpec(ctx context.Context) (*compatv1alpha1.Spec, error) {
	repo, err := remote.NewRepository(c.RegReference.String())
	if err != nil {
		return nil, err
	}
	repo.Client = c.orasClient
	repo.PlainHTTP = c.Args.PlainHttp

	opts := oras.DefaultResolveOptions
	if c.Platform != nil {
		opts.TargetPlatform = c.Platform
	}

	targetDesc, err := oras.Resolve(ctx, repo, c.RegReference.Reference, opts)
	if err != nil {
		return nil, err
	}

	descs, err := registry.Referrers(ctx, repo, targetDesc, compatv1alpha1.ArtifactType)
	if err != nil {
		return nil, nil
	} else if len(descs) < 1 {
		return nil, fmt.Errorf("compatibility artifact not found")
	}

	// Sort the artifacts in desc order.
	// If the artifact does not have creation timestamp it will be moved to the top of the slice.
	slices.SortFunc(descs, func(i, j ocispec.Descriptor) int {
		it, _ := time.Parse(time.RFC3339, i.Annotations[ArtifactCreationTimestampKey])
		jt, _ := time.Parse(time.RFC3339, j.Annotations[ArtifactCreationTimestampKey])
		return it.Compare(jt)
	})
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

	_, compatSpecRaw, err := oras.FetchBytes(ctx, repo.Blobs(), specDesc.Digest.String(), oras.DefaultFetchBytesOptions)
	if err != nil {
		return nil, err
	}

	compatSpec := compatv1alpha1.Spec{}
	err = yaml.Unmarshal(compatSpecRaw, &compatSpec)
	if err != nil {
		return nil, err
	}

	return &compatSpec, nil
}

// NodeValidatorOpts applies certain options to the node validator.
type ArtifactClientOpts interface {
	apply(*Client)
}

type artifactClientOpt struct {
	f func(*Client)
}

func (o *artifactClientOpt) apply(nv *Client) {
	o.f(nv)
}

// WithArgs applies arguments to the artifact client.
func WithArgs(args Args) ArtifactClientOpts {
	return &artifactClientOpt{f: func(c *Client) { c.Args = args }}
}

// WithPlatform applies OCI platform spec to the artifact client.
func WithPlatform(platform *ocispec.Platform) ArtifactClientOpts {
	return &artifactClientOpt{f: func(c *Client) { c.Platform = platform }}
}

// WithAuthPassword initializes oras client with user and password.
func WithAuthPassword(username, password string) ArtifactClientOpts {
	return &artifactClientOpt{f: func(c *Client) {
		c.orasClient = &auth.Client{
			Client: retry.DefaultClient,
			Cache:  auth.NewCache(),
			Credential: auth.StaticCredential(c.RegReference.Registry, auth.Credential{
				Username: username,
				Password: password,
			}),
		}
	}}
}

// WithAuthToken initializes oras client with auth token.
func WithAuthToken(token string) ArtifactClientOpts {
	return &artifactClientOpt{f: func(c *Client) {
		c.orasClient = &auth.Client{
			Client: retry.DefaultClient,
			Cache:  auth.NewCache(),
			Credential: auth.StaticCredential(c.RegReference.Registry, auth.Credential{
				AccessToken: token,
			}),
		}
	}}
}

// WithAuthDefault initializes the default oras client that does not authenticate.
func WithAuthDefault() ArtifactClientOpts {
	return &artifactClientOpt{f: func(c *Client) { c.orasClient = auth.DefaultClient }}
}
