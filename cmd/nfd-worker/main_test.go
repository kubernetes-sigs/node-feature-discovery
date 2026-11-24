/*
Copyright 2019-2021 The Kubernetes Authors.

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
	"flag"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	worker "sigs.k8s.io/node-feature-discovery/pkg/nfd-worker"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/httpserver"
)

func TestParseArgs(t *testing.T) {
	Convey("When parsing command line arguments", t, func() {
		flags := flag.NewFlagSet(ProgramName, flag.ExitOnError)

		Convey("When no override args are specified", func() {
			args := parseArgs(flags, "-oneshot")

			Convey("overrides should be nil", func() {
				So(args.Oneshot, ShouldBeTrue)
				So(args.Overrides.NoPublish, ShouldBeNil)
				So(args.Overrides.NoOwnerRefs, ShouldBeNil)
				So(args.Overrides.OwnerRefs, ShouldBeNil)
				So(args.Overrides.FeatureSources, ShouldBeNil)
				So(args.Overrides.LabelSources, ShouldBeNil)
			})

			Convey("the metrics endpoint serves plain HTTP", func() {
				So(args.TLS, ShouldResemble, httpserver.TLSFiles{})
			})
		})

		Convey("When the TLS flags are specified", func() {
			args := parseArgs(flags, "-tls-cert-file=/certs/tls.crt", "-tls-private-key-file=/certs/tls.key")

			Convey("args.TLS names both files", func() {
				So(args.TLS, ShouldResemble, httpserver.TLSFiles{CertFile: "/certs/tls.crt", KeyFile: "/certs/tls.key"})
			})
		})

		Convey("When all override args are specified", func() {
			args := parseArgs(flags,
				"-no-publish",
				"-owner-refs=ds,node",
				"-feature-sources=cpu",
				"-label-sources=fake1,fake2,fake3")

			Convey("args.sources is set to appropriate values", func() {
				So(args.Oneshot, ShouldBeFalse)
				So(*args.Overrides.NoPublish, ShouldBeTrue)
				So(args.Overrides.NoOwnerRefs, ShouldBeNil)
				So(*args.Overrides.OwnerRefs, ShouldResemble, worker.OwnerRefSources{"node", "ds"})
				So(*args.Overrides.FeatureSources, ShouldResemble, utils.StringSliceVal{"cpu"})
				So(*args.Overrides.LabelSources, ShouldResemble, utils.StringSliceVal{"fake1", "fake2", "fake3"})
			})
		})

		Convey("When the deprecated no-owner-refs override is specified", func() {
			args := parseArgs(flags, "-no-owner-refs")
			So(*args.Overrides.NoOwnerRefs, ShouldBeTrue)
			So(args.Overrides.OwnerRefs, ShouldBeNil)
		})
	})
}

func TestOwnerRefsFlagValidation(t *testing.T) {
	flags := flag.NewFlagSet(ProgramName, flag.ContinueOnError)
	_, _ = initFlags(flags)
	if got := flags.Lookup("owner-refs").DefValue; got != "pod,ds" {
		t.Fatalf("owner-refs default = %q, want %q", got, "pod,ds")
	}
	if err := flags.Parse([]string{"-owner-refs=node,invalid"}); err == nil {
		t.Fatal("expected invalid owner reference source to be rejected")
	}
}
