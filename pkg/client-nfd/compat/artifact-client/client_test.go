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

package compat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	. "github.com/smartystreets/goconvey/convey"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

// newFakeRegistry returns a registry that resolves repo:tag and answers every
// referrers request with referrersStatus.
func newFakeRegistry(referrersStatus int) *httptest.Server {
	body := []byte(`{"schemaVersion":2,"mediaType":"` + ocispec.MediaTypeImageManifest + `"}`)
	manifest := content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, body)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/repo/manifests/tag":
			w.Header().Set("Content-Type", manifest.MediaType)
			w.Header().Set("Docker-Content-Digest", manifest.Digest.String())
			w.Header().Set("Content-Length", strconv.FormatInt(manifest.Size, 10))
		case strings.HasPrefix(r.URL.Path, "/v2/repo/referrers/"):
			w.WriteHeader(referrersStatus)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestFetchCompatibilitySpec(t *testing.T) {
	Convey("When the registry rejects the referrers request", t, func() {
		reg := newFakeRegistry(http.StatusForbidden)
		defer reg.Close()

		ref, err := registry.ParseReference(reg.Listener.Addr().String() + "/repo:tag")
		So(err, ShouldBeNil)

		client := New(&ref, WithArgs(Args{PlainHttp: true}), WithAuthDefault())
		spec, err := client.FetchCompatibilitySpec(context.Background())

		Convey("The referrers error is returned", func() {
			var errResp *errcode.ErrorResponse
			So(errors.As(err, &errResp), ShouldBeTrue)
			So(errResp.StatusCode, ShouldEqual, http.StatusForbidden)
			So(errResp.URL.Path, ShouldStartWith, "/v2/repo/referrers/")
			So(spec, ShouldBeNil)
		})
	})
}
