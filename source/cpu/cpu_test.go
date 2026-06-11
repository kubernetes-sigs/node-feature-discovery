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

package cpu

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/klauspost/cpuid/v2"
	"github.com/stretchr/testify/assert"
)

func TestCpuSource(t *testing.T) {
	assert.Equal(t, src.Name(), Name)

	// Check that GetLabels works with empty features
	src.features = nil
	l, err := src.GetLabels()

	assert.Nil(t, err, err)
	assert.Empty(t, l)
}

func TestGetCpuidFlags_X86_64Levels(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("x86-64 microarchitecture levels only apply on amd64")
	}

	flags := getCpuidFlags()
	flagSet := make(map[string]bool, len(flags))
	for _, f := range flags {
		flagSet[f] = true
	}

	// V(i) must be present iff i <= the detected level. Asserting equality against
	// X64Level pins both presence and absence; a cumulative-only check would hold by
	// construction and miss an over-claimed level above what X64Level reports.
	level := cpuid.CPU.X64Level()
	for i := 1; i <= 4; i++ {
		flag := fmt.Sprintf("X86_64_V%d", i)
		assert.Equal(t, i <= level, flagSet[flag], "flag %s vs detected level %d", flag, level)
	}
}

func TestX86_64LevelFlags(t *testing.T) {
	// Deterministic, host-independent check of the level-to-flags mapping. Unlike
	// the test above, this catches an over-claimed level (e.g. a hardcoded 1..4
	// loop) on any machine, since the expected flags don't depend on the host CPU.
	cases := []struct {
		level int
		want  []string
	}{
		{level: -1, want: nil},
		{level: 0, want: nil},
		{level: 1, want: []string{"X86_64_V1"}},
		{level: 2, want: []string{"X86_64_V1", "X86_64_V2"}},
		{level: 3, want: []string{"X86_64_V1", "X86_64_V2", "X86_64_V3"}},
		{level: 4, want: []string{"X86_64_V1", "X86_64_V2", "X86_64_V3", "X86_64_V4"}},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, microarchLevelFlags(tc.level), "level %d", tc.level)
	}
}
