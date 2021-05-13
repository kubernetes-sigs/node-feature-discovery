/*
Copyright 2020 The Kubernetes Authors.

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

package nfdtopologyupdater_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	v1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	. "github.com/smartystreets/goconvey/convey"
	"k8s.io/apimachinery/pkg/util/intstr"
	nfdmaster "sigs.k8s.io/node-feature-discovery/pkg/nfd-master"
	u "sigs.k8s.io/node-feature-discovery/pkg/nfd-topology-updater"
	"sigs.k8s.io/node-feature-discovery/test/data"
)

type testContext struct {
	master nfdmaster.NfdMaster
	errs   chan error
}

func setupTest(args *nfdmaster.Args) testContext {
	// Fixed port and no-publish, for convenience
	args.NoPublish = true
	args.Port = 8192
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

func TestNewTopologyUpdater(t *testing.T) {
	Convey("When initializing new NfdTopologyUpdater instance", t, func() {
		Convey("When one of --cert-file, --key-file or --ca-file is missing", func() {
			tmPolicy := "fake-topology-manager-policy"
			_, err := u.NewTopologyUpdater(u.Args{CertFile: "crt", KeyFile: "key"}, tmPolicy)
			_, err2 := u.NewTopologyUpdater(u.Args{KeyFile: "key", CaFile: "ca"}, tmPolicy)
			_, err3 := u.NewTopologyUpdater(u.Args{CertFile: "crt", CaFile: "ca"}, tmPolicy)
			Convey("An error should be returned", func() {
				So(err, ShouldNotBeNil)
				So(err2, ShouldNotBeNil)
				So(err3, ShouldNotBeNil)
			})
		})
	})
}

func TestUpdate(t *testing.T) {
	ctx := setupTest(&nfdmaster.Args{})
	resourceInfo := v1alpha1.ResourceInfoList{
		v1alpha1.ResourceInfo{
			Name:        "cpu",
			Allocatable: intstr.FromString("2"),
			Capacity:    intstr.FromString("4"),
		},
	}
	zones := v1alpha1.ZoneList{
		v1alpha1.Zone{
			Name:      "node-0",
			Type:      "Node",
			Resources: resourceInfo,
		},
	}
	defer teardownTest(ctx)
	Convey("When running nfd-topology-updater against nfd-master", t, func() {
		Convey("When running as a Oneshot job with Zones", func() {
			updater, _ := u.NewTopologyUpdater(u.Args{Oneshot: true, Server: "localhost:8192"}, "fake-topology-manager-policy")
			err := updater.Update(zones)
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestRunTls(t *testing.T) {
	masterArgs := &nfdmaster.Args{
		CaFile:         data.FilePath("ca.crt"),
		CertFile:       data.FilePath("nfd-test-master.crt"),
		KeyFile:        data.FilePath("nfd-test-master.key"),
		VerifyNodeName: false,
	}
	ctx := setupTest(masterArgs)
	defer teardownTest(ctx)
	Convey("When running nfd-worker against nfd-master with mutual TLS auth enabled", t, func() {
		Convey("When publishing CRDs obtained from Zones", func() {
			resourceInfo := v1alpha1.ResourceInfoList{
				v1alpha1.ResourceInfo{
					Name:        "cpu",
					Allocatable: intstr.FromString("2"),
					Capacity:    intstr.FromString("4"),
				},
			}
			zones := v1alpha1.ZoneList{
				v1alpha1.Zone{
					Name:      "node-0",
					Type:      "Node",
					Resources: resourceInfo,
				},
			}
			updaterArgs := u.Args{
				CaFile:             data.FilePath("ca.crt"),
				CertFile:           data.FilePath("nfd-test-topology-updater.crt"),
				KeyFile:            data.FilePath("nfd-test-topology-updater.key"),
				Oneshot:            true,
				Server:             "localhost:8192",
				ServerNameOverride: "nfd-test-master"}
			updater, _ := u.NewTopologyUpdater(updaterArgs, "fake-topology-manager-policy")
			err := updater.Update(zones)
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}
