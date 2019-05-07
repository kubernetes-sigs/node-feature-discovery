/*
Copyright 2019 The Kubernetes Authors.

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
	nfdmaster "sigs.k8s.io/node-feature-discovery/pkg/nfd-master"
	w "sigs.k8s.io/node-feature-discovery/pkg/nfd-worker"
	"sigs.k8s.io/node-feature-discovery/test/data"
)

type testContext struct {
	master nfdmaster.NfdMaster
	errs   chan error
}

func setupTest(args nfdmaster.Args) testContext {
	// Fixed port and no-publish, for convenience
	args.NoPublish = true
	args.Port = 8192
	args.LabelWhiteList = regexp.MustCompile("")
	m, err := nfdmaster.NewNfdMaster(args)
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
	ready := ctx.master.WaitForReady(time.Second)
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
		Convey("When one of --cert-file, --key-file or --ca-file is missing", func() {
			_, err := w.NewNfdWorker(w.Args{CertFile: "crt", KeyFile: "key"})
			_, err2 := w.NewNfdWorker(w.Args{KeyFile: "key", CaFile: "ca"})
			_, err3 := w.NewNfdWorker(w.Args{CertFile: "crt", CaFile: "ca"})
			Convey("An error should be returned", func() {
				So(err, ShouldNotBeNil)
				So(err2, ShouldNotBeNil)
				So(err3, ShouldNotBeNil)
			})
		})
	})
}

func TestRun(t *testing.T) {
	ctx := setupTest(nfdmaster.Args{})
	defer teardownTest(ctx)
	Convey("When running nfd-worker against nfd-master", t, func() {
		Convey("When publishing features from fake source", func() {
			worker, _ := w.NewNfdWorker(w.Args{Oneshot: true, Sources: []string{"fake"}, Server: "localhost:8192"})
			err := worker.Run()
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestRunTls(t *testing.T) {
	masterArgs := nfdmaster.Args{
		CaFile:         data.FilePath("ca.crt"),
		CertFile:       data.FilePath("nfd-test-master.crt"),
		KeyFile:        data.FilePath("nfd-test-master.key"),
		VerifyNodeName: false,
	}
	ctx := setupTest(masterArgs)
	defer teardownTest(ctx)
	Convey("When running nfd-worker against nfd-master with mutual TLS auth enabled", t, func() {
		Convey("When publishing features from fake source", func() {
			workerArgs := w.Args{
				CaFile:             data.FilePath("ca.crt"),
				CertFile:           data.FilePath("nfd-test-worker.crt"),
				KeyFile:            data.FilePath("nfd-test-worker.key"),
				Oneshot:            true,
				Sources:            []string{"fake"},
				Server:             "localhost:8192",
				ServerNameOverride: "nfd-test-master"}
			worker, _ := w.NewNfdWorker(workerArgs)
			err := worker.Run()
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}
