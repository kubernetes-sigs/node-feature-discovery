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

package nfdworker_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/retry"
	fakenfdclient "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned/fake"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	worker "sigs.k8s.io/node-feature-discovery/pkg/nfd-worker"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

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
