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
	"flag"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"

	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

func TestParseArgs(t *testing.T) {
	Convey("When parsing command line arguments", t, func() {
		flags := flag.NewFlagSet(ProgramName, flag.ExitOnError)

		Convey("When no override args are specified", func() {
			args := parseArgs(flags, "")

			Convey("overrides should be nil", func() {
				So(args.Overrides.ExtraLabelNs, ShouldBeNil)
				So(args.Overrides.LabelWhiteList, ShouldBeNil)
				So(args.Overrides.EnableTaints, ShouldBeNil)
				So(args.Overrides.NoPublish, ShouldBeNil)
				So(args.Overrides.ResyncPeriod, ShouldBeNil)
				So(args.Overrides.InformerPageSize, ShouldBeNil)
			})
		})

		Convey("When all override args are specified", func() {
			args := parseArgs(flags,
				"-no-publish",
				"-resync-period=60s",
				"-informer-page-size=100",
				"-extra-label-ns=ns",
				"-deny-label-ns=denied",
				"-label-whitelist=foo",
				"-enable-taints",
				"-nfd-api-parallelism=5")

			Convey("args.sources is set to appropriate values", func() {
				So(*args.Overrides.ExtraLabelNs, ShouldEqual, utils.StringSetVal{"ns": struct{}{}})
				So(*args.Overrides.DenyLabelNs, ShouldEqual, utils.StringSetVal{"denied": struct{}{}})
				So(args.Overrides.LabelWhiteList.String(), ShouldEqual, "foo")
				So(*args.Overrides.EnableTaints, ShouldBeTrue)
				So(*args.Overrides.NoPublish, ShouldBeTrue)
				So(*args.Overrides.ResyncPeriod, ShouldEqual, utils.DurationVal{Duration: time.Duration(60000000000)})
				So(*args.Overrides.InformerPageSize, ShouldEqual, 100)
				So(*args.Overrides.NfdApiParallelism, ShouldEqual, 5)
			})
		})
	})
}
