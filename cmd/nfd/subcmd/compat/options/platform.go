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

package options

import (
	"fmt"
	"runtime"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
)

// PlatformOption represents
type PlatformOption struct {
	// PlatformStr contains the raw platform argument provided by the user.
	PlatformStr string
	// Platform represents the OCI platform specification, built from PlatformStr.
	Platform *ocispec.Platform
}

// Parse takes the PlatformStr argument provided by the user
// to build OCI platform specification.
func (opt *PlatformOption) Parse(*cobra.Command) error {
	var pStr string

	if opt.PlatformStr == "" {
		return nil
	}

	platform := &ocispec.Platform{}
	pStr, platform.OSVersion, _ = strings.Cut(opt.PlatformStr, ":")
	parts := strings.Split(pStr, "/")

	switch len(parts) {
	case 3:
		platform.Variant = parts[2]
		fallthrough
	case 2:
		platform.Architecture = parts[1]
	case 1:
		platform.Architecture = runtime.GOARCH
	default:
		return fmt.Errorf("failed to parse platform %q: expected format os[/arch[/variant]]", opt.PlatformStr)
	}

	platform.OS = parts[0]
	if platform.OS == "" {
		return fmt.Errorf("invalid platform: OS cannot be empty")
	}
	if platform.Architecture == "" {
		return fmt.Errorf("invalid platform: Architecture cannot be empty")
	}
	opt.Platform = platform
	return nil
}
