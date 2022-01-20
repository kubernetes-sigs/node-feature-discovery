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

package topologyupdater_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	v1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	. "github.com/smartystreets/goconvey/convey"
	"k8s.io/apimachinery/pkg/api/resource"
	nfdclient "sigs.k8s.io/node-feature-discovery/pkg/nfd-client"
	u "sigs.k8s.io/node-feature-discovery/pkg/nfd-client/topology-updater"
	nfdmaster "sigs.k8s.io/node-feature-discovery/pkg/nfd-master"
	"sigs.k8s.io/node-feature-discovery/pkg/resourcemonitor"
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

func TestNewTopologyUpdater(t *testing.T) {
	Convey("When initializing new NfdTopologyUpdater instance", t, func() {
		Convey("When one of -cert-file, -key-file or -ca-file is missing", func() {
			tmPolicy := "fake-topology-manager-policy"
			_, err := u.NewTopologyUpdater(u.Args{Args: nfdclient.Args{CertFile: "crt", KeyFile: "key"}}, resourcemonitor.Args{}, tmPolicy)
			_, err2 := u.NewTopologyUpdater(u.Args{Args: nfdclient.Args{KeyFile: "key", CaFile: "ca"}}, resourcemonitor.Args{}, tmPolicy)
			_, err3 := u.NewTopologyUpdater(u.Args{Args: nfdclient.Args{CertFile: "crt", CaFile: "ca"}}, resourcemonitor.Args{}, tmPolicy)
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
			Available:   resource.MustParse("2"),
			Allocatable: resource.MustParse("4"),
			Capacity:    resource.MustParse("4"),
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
			args := u.Args{
				Oneshot: true,
				Args: nfdclient.Args{
					Server: "localhost:8192"},
			}
			updater, _ := u.NewTopologyUpdater(args, resourcemonitor.Args{}, "fake-topology-manager-policy")
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
		Convey("When publishing CRs obtained from Zones", func() {
			resourceInfo := v1alpha1.ResourceInfoList{
				v1alpha1.ResourceInfo{
					Name:        "cpu",
					Available:   resource.MustParse("2"),
					Allocatable: resource.MustParse("4"),
					Capacity:    resource.MustParse("4"),
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
				Args: nfdclient.Args{
					CaFile:             data.FilePath("ca.crt"),
					CertFile:           data.FilePath("nfd-test-topology-updater.crt"),
					KeyFile:            data.FilePath("nfd-test-topology-updater.key"),
					Server:             "localhost:8192",
					ServerNameOverride: "nfd-test-master",
				},
				Oneshot: true,
			}

			updater, _ := u.NewTopologyUpdater(updaterArgs, resourcemonitor.Args{}, "fake-topology-manager-policy")
			err := updater.Update(zones)
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}
