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

package kernel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

func TestKernelSource(t *testing.T) {
	assert.Equal(t, src.Name(), Name)

	// Check that GetLabels works with empty features
	src.features = nil
	l, err := src.GetLabels()

	assert.Nil(t, err, err)
	assert.Empty(t, l)

}

func TestDiscoverEnabledModules(t *testing.T) {
	loaded, err := getLoadedModules()
	if err != nil || len(loaded) == 0 {
		t.Skip("no loaded kernel modules readable from /proc/modules")
	}
	kVersion, err := getVersion()
	require.NoError(t, err)

	// Discover() fills the package-level src; restore it so that other tests
	// (e.g. TestKernelSource with -count=2) start from the same state.
	origLibDir := hostpath.LibDir
	origFeatures, origLegacyKconfig := src.features, src.legacyKconfig
	t.Cleanup(func() {
		hostpath.LibDir = origLibDir
		src.features = origFeatures
		src.legacyKconfig = origLegacyKconfig
	})

	t.Run("without modules.builtin", func(t *testing.T) {
		hostpath.LibDir = hostpath.HostDir(t.TempDir())

		require.NoError(t, src.Discover())
		enabled := src.GetFeatures().Flags[EnabledModuleFeature].Elements
		for _, m := range loaded {
			assert.Contains(t, enabled, m)
		}
	})

	t.Run("with modules.builtin", func(t *testing.T) {
		libDir := t.TempDir()
		modDir := filepath.Join(libDir, "modules", kVersion)
		require.NoError(t, os.MkdirAll(modDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(modDir, "modules.builtin"),
			[]byte("kernel/drivers/test/nfd_test_builtin.ko\n"), 0o644))
		hostpath.LibDir = hostpath.HostDir(libDir)

		require.NoError(t, src.Discover())
		enabled := src.GetFeatures().Flags[EnabledModuleFeature].Elements
		assert.Contains(t, enabled, "nfd_test_builtin")
		for _, m := range loaded {
			assert.Contains(t, enabled, m)
		}
	})
}
