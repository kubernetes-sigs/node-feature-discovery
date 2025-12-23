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

package nfdmaster_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	m "sigs.k8s.io/node-feature-discovery/pkg/nfd-master"
)

func TestNewNfdMaster(t *testing.T) {
	Convey("When initializing new NfdMaster instance", t, func() {
		Convey("When -config is supplied", func() {
			//nolint:staticcheck // See issue #2400 for migration to NewClientset
			k8sCli := fakeclient.NewSimpleClientset()
			_, err := m.NewNfdMaster(
				m.WithArgs(&m.Args{
					ConfigFile: "master-config.yaml",
				}),
				m.WithKubernetesClient(k8sCli))
			Convey("An error should not be returned", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}
