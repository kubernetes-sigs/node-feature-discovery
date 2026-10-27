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
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	fakenfdclient "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned/fake"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/features"
	worker "sigs.k8s.io/node-feature-discovery/pkg/nfd-worker"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

var initializeFeatureGatesOnce sync.Once

func initializeFeatureGates() {
	initializeFeatureGatesOnce.Do(func() {
		if err := features.NFDMutableFeatureGate.Add(features.DefaultNFDFeatureGates); err != nil {
			klog.ErrorS(err, "failed to add default feature gates")
			os.Exit(1)
		}
	})
}

func TestOwnerReferenceMigrationWithUnchangedSpec(t *testing.T) {
	initializeFeatureGates()
	t.Setenv("NODE_NAME", "fake-node")
	t.Setenv("KUBERNETES_NAMESPACE", "fake-ns")
	t.Setenv("POD_NAME", "fake-worker")
	t.Setenv("POD_UID", "fake-worker-pod-uid")

	//nolint:staticcheck // NewClientset cannot field-manage NodeFeature with the generated schema.
	nfdCli := fakenfdclient.NewSimpleClientset()
	k8sCli := fakeclient.NewSimpleClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "fake-node", UID: types.UID("node-uid")},
	})

	emptyOwnerRefs := worker.OwnerRefSources{}
	initialArgs := &worker.Args{
		Oneshot: true,
		Overrides: worker.ConfigOverrideArgs{
			FeatureSources: &utils.StringSliceVal{"fake"},
			LabelSources:   &utils.StringSliceVal{"fake"},
			OwnerRefs:      &emptyOwnerRefs,
		},
	}
	initialWorker, err := worker.NewNfdWorker(
		worker.WithArgs(initialArgs),
		worker.WithKubernetesClient(k8sCli),
		worker.WithNFDClient(nfdCli),
	)
	if err != nil {
		t.Fatalf("failed to create initial worker: %v", err)
	}
	if err := initialWorker.Run(); err != nil {
		t.Fatalf("initial worker run failed: %v", err)
	}
	initialNF, err := nfdCli.NfdV1alpha1().NodeFeatures("fake-ns").Get(context.TODO(), "fake-node", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get initial NodeFeature: %v", err)
	}

	nodeOwnerRefs := worker.OwnerRefSources{"node"}
	migratingArgs := &worker.Args{
		Oneshot: true,
		Overrides: worker.ConfigOverrideArgs{
			FeatureSources: &utils.StringSliceVal{"fake"},
			LabelSources:   &utils.StringSliceVal{"fake"},
			OwnerRefs:      &nodeOwnerRefs,
		},
	}
	migratingWorker, err := worker.NewNfdWorker(
		worker.WithArgs(migratingArgs),
		worker.WithKubernetesClient(k8sCli),
		worker.WithNFDClient(nfdCli),
	)
	if err != nil {
		t.Fatalf("failed to create migrating worker: %v", err)
	}
	if err := migratingWorker.Run(); err != nil {
		t.Fatalf("migrating worker run failed: %v", err)
	}
	updatedNF, err := nfdCli.NfdV1alpha1().NodeFeatures("fake-ns").Get(context.TODO(), "fake-node", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get migrated NodeFeature: %v", err)
	}

	Convey("owner references migrate without changing the discovered spec", t, func() {
		So(updatedNF.Spec, ShouldResemble, initialNF.Spec)
		So(updatedNF.OwnerReferences, ShouldResemble, []metav1.OwnerReference{
			{
				APIVersion:         "v1",
				Kind:               "Node",
				Name:               "fake-node",
				UID:                "node-uid",
				BlockOwnerDeletion: new(false),
			},
		})
	})
}

func TestRunNodeFeatureUpdateConflicts(t *testing.T) {
	initializeFeatureGates()
	for _, tc := range []struct {
		name         string
		conflicts    int
		forbidden    bool
		wantAttempts int
	}{
		{name: "re-fetch after conflict", conflicts: 1, wantAttempts: 2},
		{name: "bounded conflict retries", conflicts: retry.DefaultRetry.Steps, wantAttempts: retry.DefaultRetry.Steps},
		{name: "do not retry forbidden", forbidden: true, wantAttempts: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NODE_NAME", "fake-node")
			t.Setenv("KUBERNETES_NAMESPACE", "fake-ns")
			t.Setenv("POD_NAME", "replacement-worker")
			t.Setenv("POD_UID", "replacement-pod-uid")

			dsOwner := metav1.OwnerReference{APIVersion: "apps/v1", Kind: "DaemonSet", Name: "fake-worker", UID: "ds-uid", Controller: new(true), BlockOwnerDeletion: new(false)}
			initial := &nfdv1alpha1.NodeFeature{ObjectMeta: metav1.ObjectMeta{
				Name: "fake-node", Namespace: "fake-ns", ResourceVersion: "1",
				OwnerReferences: []metav1.OwnerReference{
					{APIVersion: "v1", Kind: "Pod", Name: "old-worker", UID: "old-pod-uid"},
					dsOwner,
				},
			}}
			//nolint:staticcheck // NewClientset cannot field-manage NodeFeature with the generated schema.
			nfdCli := fakenfdclient.NewSimpleClientset(initial)
			k8sCli := fakeclient.NewClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name: "replacement-worker", Namespace: "fake-ns", OwnerReferences: []metav1.OwnerReference{dsOwner},
			}})
			resource := nfdv1alpha1.SchemeGroupVersion.WithResource("nodefeatures")
			gets, updates := 0, 0
			nfdCli.PrependReactor("get", "nodefeatures", func(clienttesting.Action) (bool, runtime.Object, error) {
				gets++
				return false, nil, nil
			})
			nfdCli.PrependReactor("update", "nodefeatures", func(action clienttesting.Action) (bool, runtime.Object, error) {
				updates++
				if tc.forbidden {
					return true, nil, apierrors.NewForbidden(resource.GroupResource(), "fake-node", fmt.Errorf("denied"))
				}
				if updates == 1 {
					// The real API enforces resource versions; the fake tracker does
					// not. Model GC removing the old Pod owner after the worker's GET.
					latest := initial.DeepCopy()
					latest.ResourceVersion = "2"
					latest.OwnerReferences = []metav1.OwnerReference{dsOwner}
					if err := nfdCli.Tracker().Update(resource, latest, "fake-ns"); err != nil {
						t.Fatalf("simulate concurrent owner-reference update: %v", err)
					}
				}
				update := action.(clienttesting.UpdateAction).GetObject().(*nfdv1alpha1.NodeFeature)
				if updates <= tc.conflicts || update.ResourceVersion != "2" {
					return true, nil, apierrors.NewConflict(resource.GroupResource(), "fake-node", fmt.Errorf("the object has been modified"))
				}
				return false, nil, nil
			})

			args := &worker.Args{Oneshot: true, Overrides: worker.ConfigOverrideArgs{
				FeatureSources: &utils.StringSliceVal{"fake"}, LabelSources: &utils.StringSliceVal{"fake"},
			}}
			w, err := worker.NewNfdWorker(worker.WithArgs(args), worker.WithKubernetesClient(k8sCli), worker.WithNFDClient(nfdCli))
			if err != nil {
				t.Fatalf("create worker: %v", err)
			}
			err = w.Run()
			switch {
			case tc.forbidden:
				if !apierrors.IsForbidden(err) {
					t.Fatalf("expected Forbidden error, got %v", err)
				}
			case tc.conflicts == retry.DefaultRetry.Steps:
				if !apierrors.IsConflict(err) {
					t.Fatalf("expected Conflict after retry exhaustion, got %v", err)
				}
			default:
				if err != nil {
					t.Fatalf("worker must recover from a transient update conflict: %v", err)
				}
			}
			if gets != tc.wantAttempts || updates != tc.wantAttempts {
				t.Fatalf("got %d GETs and %d UPDATEs, want %d of each", gets, updates, tc.wantAttempts)
			}
			if err != nil {
				return
			}
			nf, err := nfdCli.NfdV1alpha1().NodeFeatures("fake-ns").Get(context.Background(), "fake-node", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("read published NodeFeature: %v", err)
			}
			wantLabels := map[string]string{
				"feature.node.kubernetes.io/fake-fakefeature1": "true",
				"feature.node.kubernetes.io/fake-fakefeature2": "true",
				"feature.node.kubernetes.io/fake-fakefeature3": "true",
			}
			if !reflect.DeepEqual(nf.Spec.Labels, wantLabels) || len(nf.Spec.Features.Instances["fake.instance"].Elements) != 3 {
				t.Fatalf("discovered features were not published after retry: %#v", nf.Spec)
			}
			wantOwners := []metav1.OwnerReference{
				{APIVersion: "v1", Kind: "Pod", Name: "replacement-worker", UID: "replacement-pod-uid", BlockOwnerDeletion: new(false)},
				dsOwner,
			}
			if !reflect.DeepEqual(nf.OwnerReferences, wantOwners) || nf.ResourceVersion != "2" {
				t.Fatalf("unexpected owners or stale resource version after retry: %#v", nf.ObjectMeta)
			}
		})
	}
}

func TestRun(t *testing.T) {
	//nolint:staticcheck
	nfdCli := fakenfdclient.NewSimpleClientset()
	initializeFeatureGates()
	Convey("When running nfd-worker", t, func() {
		Convey("When publishing features from fake source", func() {
			t.Setenv("NODE_NAME", "fake-node")
			t.Setenv("KUBERNETES_NAMESPACE", "fake-ns")
			t.Setenv("POD_NAME", "")
			t.Setenv("POD_UID", "fake-worker-pod-uid")
			args := &worker.Args{
				Oneshot: true,
				Overrides: worker.ConfigOverrideArgs{
					FeatureSources: &utils.StringSliceVal{"fake"},
					LabelSources:   &utils.StringSliceVal{"fake"},
				},
			}
			k8sCli := fakeclient.NewClientset()
			w, _ := worker.NewNfdWorker(
				worker.WithArgs(args),
				worker.WithKubernetesClient(k8sCli),
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
							"nfd.node.kubernetes.io/worker.pod.uid": "fake-worker-pod-uid",
						},
						OwnerReferences: []metav1.OwnerReference{},
					},
					Spec: nfdv1alpha1.NodeFeatureSpec{
						Labels: map[string]string{
							"feature.node.kubernetes.io/fake-fakefeature1": "true",
							"feature.node.kubernetes.io/fake-fakefeature2": "true",
							"feature.node.kubernetes.io/fake-fakefeature3": "true",
						},
						Features: nfdv1alpha1.Features{
							Flags: map[string]nfdv1alpha1.FlagFeatureSet{
								"fake.flag": {
									Elements: map[string]nfdv1alpha1.Nil{
										"flag_1": {},
										"flag_2": {},
										"flag_3": {}},
								},
							},
							Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{
								"fake.attribute": {
									Elements: map[string]string{
										"attr_1": "true",
										"attr_2": "false",
										"attr_3": "10",
									},
								},
							},
							Instances: map[string]nfdv1alpha1.InstanceFeatureSet{
								"fake.instance": {
									Elements: []nfdv1alpha1.InstanceFeature{
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
