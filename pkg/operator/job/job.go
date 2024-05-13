/*
Copyright 2024 The Kubernetes Authors.

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

package job

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
)

//go:generate mockgen -source=job.go -package=job -destination=mock_job.go JobAPI

type JobAPI interface {
	GetJob(ctx context.Context, namespace, name string) (*batchv1.Job, error)
	CreatePruneJob(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error
}

type job struct {
	client client.Client
	scheme *runtime.Scheme
}

func NewJobAPI(client client.Client, scheme *runtime.Scheme) JobAPI {
	return &job{
		client: client,
		scheme: scheme,
	}
}

func (j *job) GetJob(ctx context.Context, namespace, name string) (*batchv1.Job, error) {
	pruneJob := &batchv1.Job{}
	err := j.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, pruneJob)
	if err != nil {
		return nil, fmt.Errorf("failed to get prune job: %w", err)
	}

	return pruneJob, nil
}

func (j *job) CreatePruneJob(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error {
	pruneJob := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nfd-prune",
			Namespace: nfdInstance.Namespace,
			Labels:    map[string]string{"app": "nfd"},
		},
		Spec: batchv1.JobSpec{
			Completions: ptr.To[int32](1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "nfd-prune"},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "nfd-prune",
					Affinity:           getPodsAffinity(),
					RestartPolicy:      corev1.RestartPolicyNever,
					Tolerations:        getPodsTolerations(),
					Containers: []corev1.Container{
						{
							Name:            "nfd-prune",
							Image:           nfdInstance.Spec.Operand.ImagePath(),
							ImagePullPolicy: corev1.PullAlways,
							Command: []string{
								"nfd-master",
							},
							Args:            []string{"-prune"},
							Env:             getEnvs(),
							SecurityContext: getSecurityContext(),
						},
					},
				},
			},
		},
	}

	err := controllerutil.SetControllerReference(nfdInstance, &pruneJob, j.scheme)
	if err != nil {
		return fmt.Errorf("failed to set controller reference for prune job: %w", err)
	}

	return j.client.Create(ctx, &pruneJob)
}

func getPodsTolerations() []corev1.Toleration {
	return []corev1.Toleration{
		{
			Key:      "node-role.kubernetes.io/master",
			Operator: corev1.TolerationOpEqual,
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:      "node-role.kubernetes.io/control-plane",
			Operator: corev1.TolerationOpEqual,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}
}

func getPodsAffinity() *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				{
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "node-role.kubernetes.io/master",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{""},
							},
						},
					},
					Weight: 1,
				},
				{
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "node-role.kubernetes.io/control-plane",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{""},
							},
						},
					},
					Weight: 1,
				},
			},
		},
	}
}

func getEnvs() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}
}

func getSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsNonRoot:           ptr.To(true),
		ReadOnlyRootFilesystem: ptr.To(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		AllowPrivilegeEscalation: ptr.To(false),
	}
}
