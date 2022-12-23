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

package nfdworker_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"

	master "sigs.k8s.io/node-feature-discovery/pkg/nfd-master"
	worker "sigs.k8s.io/node-feature-discovery/pkg/nfd-worker"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/test/data"
)

type testContext struct {
	master master.NfdMaster
	errs   chan error
}

func setupTest(args *master.Args) testContext {
	// Fixed port and no-publish, for convenience
	args.NoPublish = true
	args.Port = 8192
	args.LabelWhiteList.Regexp = *regexp.MustCompile("")
	m, err := master.NewNfdMaster(args)
	if err != nil {
		fmt.Printf("Test setup failed: %v\n", err)
		os.Exit(1)
	}
	ctx := testContext{master: m, errs: make(chan error)}

	// Run nfd-master instance, intended to be used as the server counterpart
	go func() {
		ctx.errs <- ctx.master.Run()
		close(ctx.errs)
	}()
	ready := ctx.master.WaitForReady(5 * time.Second)
	if !ready {
		fmt.Println("Test setup failed: timeout while waiting for nfd-master")
		os.Exit(1)
	}

	return ctx
}

func teardownTest(ctx testContext) {
	ctx.master.Stop()
	for e := range ctx.errs {
		if e != nil {
			fmt.Printf("Error in test context: %v\n", e)
			os.Exit(1)
		}
	}
}

func TestNewNfdWorker(t *testing.T) {
	Convey("When initializing new NfdWorker instance", t, func() {
		Convey("When one of -cert-file, -key-file or -ca-file is missing", func() {
			_, err := worker.NewNfdWorker(&worker.Args{CertFile: "crt", KeyFile: "key"})
			_, err2 := worker.NewNfdWorker(&worker.Args{KeyFile: "key", CaFile: "ca"})
			_, err3 := worker.NewNfdWorker(&worker.Args{CertFile: "crt", CaFile: "ca"})
			Convey("An error should be returned", func() {
				So(err, ShouldNotBeNil)
				So(err2, ShouldNotBeNil)
				So(err3, ShouldNotBeNil)
			})
		})
	})
}

func TestRun(t *testing.T) {
	ctx := setupTest(&master.Args{})
	defer teardownTest(ctx)
	Convey("When running nfd-worker against nfd-master", t, func() {
		Convey("When publishing features from fake source", func() {
			args := &worker.Args{
				Server:    "localhost:8192",
				Oneshot:   true,
				Overrides: worker.ConfigOverrideArgs{LabelSources: &utils.StringSliceVal{"fake"}},
			}
			fooasdf, _ := worker.NewNfdWorker(args)
			err := fooasdf.Run()
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestRunTls(t *testing.T) {
	masterArgs := &master.Args{
		CaFile:         data.FilePath("ca.crt"),
		CertFile:       data.FilePath("nfd-test-master.crt"),
		KeyFile:        data.FilePath("nfd-test-master.key"),
		VerifyNodeName: false,
	}
	ctx := setupTest(masterArgs)
	defer teardownTest(ctx)
	Convey("When running nfd-worker against nfd-master with mutual TLS auth enabled", t, func() {
		Convey("When publishing features from fake source", func() {
			workerArgs := worker.Args{
				CaFile:             data.FilePath("ca.crt"),
				CertFile:           data.FilePath("nfd-test-worker.crt"),
				KeyFile:            data.FilePath("nfd-test-worker.key"),
				Server:             "localhost:8192",
				ServerNameOverride: "nfd-test-master",
				Oneshot:            true,
				Overrides:          worker.ConfigOverrideArgs{LabelSources: &utils.StringSliceVal{"fake"}},
			}
			w, _ := worker.NewNfdWorker(&workerArgs)
			err := w.Run()
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}
