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

	// Every x86-64 CPU is at least level 1
	assert.True(t, flagSet["X86_64_V1"], "expected X86_64_V1 flag on amd64")

	// Levels must be cumulative: if V(n) is present, all V(1)..V(n-1) must be too
	for level := 4; level >= 2; level-- {
		flag := fmt.Sprintf("X86_64_V%d", level)
		if !flagSet[flag] {
			continue
		}
		for i := 1; i < level; i++ {
			lower := fmt.Sprintf("X86_64_V%d", i)
			assert.True(t, flagSet[lower], "X86_64_V%d present but %s missing", level, lower)
		}
	}
}
