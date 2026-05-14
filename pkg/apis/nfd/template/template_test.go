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

package template

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	stdtemplate "text/template"
	"time"

	"github.com/stretchr/testify/require"
)

// TestMain handles a subprocess re-exec used by TestParse_LargeInputExitsQuickly.
// When NFD_TEMPLATE_LARGE_INPUT is set, this binary runs text/template.Parse
// directly on a pathologically large input, bypassing NewHelper's size cap.
// Otherwise, runs the normal test suite.
func TestMain(m *testing.M) {
	if os.Getenv("NFD_TEMPLATE_LARGE_INPUT") == "1" {
		_, _ = stdtemplate.New("").Parse(strings.Repeat("{{if 1}}", 600_000))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestNewHelper_RejectsOverBoundary(t *testing.T) {
	_, err := NewHelper(strings.Repeat("a", MaxTemplateSize+1))
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrTemplateTooLarge),
		"expected ErrTemplateTooLarge, got %v", err)
}

func TestNewHelper_AcceptsBoundary(t *testing.T) {
	// Exactly at the cap must be accepted. Input is plain text (no template
	// actions) so the parser does no recursion regardless of length.
	_, err := NewHelper(strings.Repeat("a", MaxTemplateSize))
	require.NoError(t, err)
}

func TestNewHelper_RejectsLargeInput(t *testing.T) {
	// A 600_000-action input is well over the cap. NewHelper should reject
	// it via the size guard rather than letting it through to the recursive
	// parser.
	_, err := NewHelper(strings.Repeat("{{if 1}}", 600_000))
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrTemplateTooLarge),
		"expected ErrTemplateTooLarge, got %v", err)
}

func TestParse_LargeInputExitsQuickly(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess re-exec is slow under -short")
	}
	// Subprocess re-exec: the child runs text/template.Parse directly on a
	// 600_000-action input (bypassing NewHelper's size cap). text/template
	// does not internally bound input compute, so the application-layer cap
	// is what keeps parse latency predictable. The child is expected to NOT
	// complete the parse cleanly and to exit quickly rather than hang. If a
	// future Go release changes that behavior, this test will flip and the
	// application-layer cap may become tunable rather than load-bearing.
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "NFD_TEMPLATE_LARGE_INPUT=1")
	start := time.Now()
	_, err := cmd.CombinedOutput()
	elapsed := time.Since(start)
	require.Error(t, err, "child should not complete the parse cleanly")
	require.Less(t, elapsed.Seconds(), 30.0, "child should exit quickly, not hang")
}

func TestNewHelper_DropsHostEnvFuncs(t *testing.T) {
	// env / expandenv / getHostByName read process env and outbound DNS;
	// not useful for NodeFeatureRule label expansion.
	for _, tmpl := range []string{
		`{{env "PATH"}}`,
		`{{expandenv "$PATH"}}`,
		`{{getHostByName "localhost"}}`,
	} {
		_, err := NewHelper(tmpl)
		require.Error(t, err, "expected %q to fail", tmpl)
	}
}

func TestNewHelper_KeepsUsefulSprigFuncs(t *testing.T) {
	// Sanity: sprig's general-purpose functions remain available. Picks one
	// from each major category; not exhaustive.
	for _, tmpl := range []string{
		`{{upper "x"}}`,
		`{{add 1 2}}`,
		`{{trim " x "}}`,
	} {
		_, err := NewHelper(tmpl)
		require.NoError(t, err, "expected %q to succeed", tmpl)
	}
}
