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

package main

import (
	"reflect"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
)

// The manager serves /metrics itself, so the endpoint has to use TLS and
// check the caller's token (TokenReview) and its permission to GET /metrics
// (SubjectAccessReview).
func TestMetricsServerOptions(t *testing.T) {
	// Not the flag default, so a hardcoded address fails the test.
	const addr = "127.0.0.1:18443"
	opts := metricsServerOptions(addr)

	if opts.BindAddress != addr {
		t.Errorf("BindAddress = %q, want %q", opts.BindAddress, addr)
	}
	if !opts.SecureServing {
		t.Error("SecureServing = false, want true: /metrics must be served over TLS")
	}
	if opts.FilterProvider == nil {
		t.Fatal("FilterProvider is nil: /metrics must require authentication and authorization")
	}
	got := reflect.ValueOf(opts.FilterProvider).Pointer()
	want := reflect.ValueOf(filters.WithAuthenticationAndAuthorization).Pointer()
	if got != want {
		t.Error("FilterProvider is not filters.WithAuthenticationAndAuthorization")
	}
}
