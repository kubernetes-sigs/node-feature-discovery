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
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"

	fakenfdclient "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned/fake"
	"sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/features"
	worker "sigs.k8s.io/node-feature-discovery/pkg/nfd-worker"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

func initializeFeatureGates() {
	if err := features.NFDMutableFeatureGate.Add(features.DefaultNFDFeatureGates); err != nil {
		klog.ErrorS(err, "failed to add default feature gates")
		os.Exit(1)
	}
}

func TestRun(t *testing.T) {
	nfdCli := fakenfdclient.NewSimpleClientset()
	initializeFeatureGates()
	Convey("When running nfd-worker", t, func() {
		Convey("When publishing features from fake source", func() {
			os.Setenv("NODE_NAME", "fake-node")
			os.Setenv("KUBERNETES_NAMESPACE", "fake-ns")
			b := true
			args := &worker.Args{
				Oneshot:     true,
				NoOwnerRefs: true,
				Overrides: worker.ConfigOverrideArgs{
					NoOwnerRefs:    &b,
					FeatureSources: &utils.StringSliceVal{"fake"},
					LabelSources:   &utils.StringSliceVal{"fake"},
				},
			}
			w, _ := worker.NewNfdWorker(
				worker.WithArgs(args),
				worker.WithKubernetesClient(fakeclient.NewSimpleClientset()),
				worker.WithNFDClient(nfdCli),
			)
			err := w.Run()
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
			Convey("NodeFeture object should be created", func() {
				nf, err := nfdCli.NfdV1alpha1().NodeFeatures("fake-ns").Get(context.TODO(), "fake-node", metav1.GetOptions{})
				So(err, ShouldBeNil)

				nfExpected := &nfdv1alpha1.NodeFeature{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fake-node",
						Namespace: "fake-ns",
						Labels: map[string]string{
							"nfd.node.kubernetes.io/node-name": "fake-node",
						},
						Annotations: map[string]string{
							"nfd.node.kubernetes.io/worker.version": "undefined",
						},
						OwnerReferences: []metav1.OwnerReference{},
					},
					Spec: nfdv1alpha1.NodeFeatureSpec{
						Labels: map[string]string{
							"feature.node.kubernetes.io/fake-fakefeature1": "true",
							"feature.node.kubernetes.io/fake-fakefeature2": "true",
							"feature.node.kubernetes.io/fake-fakefeature3": "true",
						},
						Features: v1alpha1.Features{
							Flags: map[string]v1alpha1.FlagFeatureSet{
								"fake.flag": {
									Elements: map[string]v1alpha1.Nil{
										"flag_1": {},
										"flag_2": {},
										"flag_3": {}},
								},
							},
							Attributes: map[string]v1alpha1.AttributeFeatureSet{
								"fake.attribute": {
									Elements: map[string]string{
										"attr_1": "true",
										"attr_2": "false",
										"attr_3": "10",
									},
								},
							},
							Instances: map[string]v1alpha1.InstanceFeatureSet{
								"fake.instance": {
									Elements: []v1alpha1.InstanceFeature{
										{Attributes: map[string]string{
											"name":   "instance_1",
											"attr_1": "true",
											"attr_2": "false",
											"attr_3": "10",
											"attr_4": "foobar",
										}},
										{Attributes: map[string]string{
											"name":   "instance_2",
											"attr_1": "true",
											"attr_2": "true",
											"attr_3": "100",
										}},
										{Attributes: map[string]string{
											"name": "instance_3",
										}},
									},
								},
							},
						},
					},
				}
				So(nf, ShouldResemble, nfExpected)
			})
		})
	})
}
