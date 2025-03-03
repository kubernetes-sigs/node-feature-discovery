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

package deployment

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
)

const (
	defaultServicePort int = 12000
)

//go:generate mockgen -source=deployment.go -package=deployment -destination=mock_deployment.go DeploymentAPI

type DeploymentAPI interface {
	SetMasterDeploymentAsDesired(nfdInstance *nfdv1.NodeFeatureDiscovery, masterDep *v1.Deployment) error
	SetGCDeploymentAsDesired(nfdInstance *nfdv1.NodeFeatureDiscovery, gcDep *v1.Deployment) error
	DeleteDeployment(ctx context.Context, namespace, name string) error
	GetDeployment(ctx context.Context, namespace, name string) (*v1.Deployment, error)
}

type deployment struct {
	client client.Client
	scheme *runtime.Scheme
}

func NewDeploymentAPI(client client.Client, scheme *runtime.Scheme) DeploymentAPI {
	return &deployment{
		client: client,
		scheme: scheme,
	}
}

func (d *deployment) SetMasterDeploymentAsDesired(nfdInstance *nfdv1.NodeFeatureDiscovery, masterDep *v1.Deployment) error {
	standartLabels := map[string]string{"app": "nfd-master"}
	masterDep.ObjectMeta.Labels = standartLabels

	masterDep.Spec = v1.DeploymentSpec{
		Replicas: ptr.To[int32](1),
		Selector: &metav1.LabelSelector{
			MatchLabels: standartLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: standartLabels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "nfd-master",
				DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
				RestartPolicy:      corev1.RestartPolicyAlways,
				Tolerations:        getPodsTolerations(nfdInstance),
				Affinity:           getPodsAffinity(),
				Containers: []corev1.Container{
					{
						Name:            "nfd-master",
						Image:           nfdInstance.Spec.Operand.ImagePath(),
						ImagePullPolicy: getImagePullPolicy(nfdInstance),
						Command: []string{
							"nfd-master",
						},
						Args:            getArgs(nfdInstance),
						Env:             getEnvs(),
						SecurityContext: getMasterSecurityContext(),
						LivenessProbe:   getLivenessProbe(),
						ReadinessProbe:  getReadinessProbe(),
					},
				},
			},
		},
	}
	return controllerutil.SetControllerReference(nfdInstance, masterDep, d.scheme)
}

func (d *deployment) SetGCDeploymentAsDesired(nfdInstance *nfdv1.NodeFeatureDiscovery, gcDep *v1.Deployment) error {
	gcDep.ObjectMeta.Labels = map[string]string{"app": "nfd"}
	matchLabels := map[string]string{"app": "nfd-gc"}
	gcDep.Spec = v1.DeploymentSpec{
		Replicas: ptr.To[int32](1),
		Selector: &metav1.LabelSelector{
			MatchLabels: matchLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: matchLabels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "nfd-gc",
				DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
				RestartPolicy:      corev1.RestartPolicyAlways,
				Containers: []corev1.Container{
					{
						Name:            "nfd-gc",
						Image:           nfdInstance.Spec.Operand.ImagePath(),
						ImagePullPolicy: corev1.PullAlways,
						Command: []string{
							"nfd-gc",
						},
						Env:             getEnvs(),
						SecurityContext: getGCSecurityContext(),
					},
				},
			},
		},
	}
	return controllerutil.SetControllerReference(nfdInstance, gcDep, d.scheme)
}

func (d *deployment) DeleteDeployment(ctx context.Context, namespace, name string) error {
	dep := v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	err := d.client.Delete(ctx, &dep)
	if err != nil && client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete deployment %s/%s: %w", namespace, name, err)
	}
	return nil
}

func (d *deployment) GetDeployment(ctx context.Context, namespace, name string) (*v1.Deployment, error) {
	dep := &v1.Deployment{}
	err := d.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, dep)
	return dep, err
}

func getPodsTolerations(nfdInstance *nfdv1.NodeFeatureDiscovery) []corev1.Toleration {
	basicTolerations := []corev1.Toleration{
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

	return append(basicTolerations, nfdInstance.Spec.Operand.MasterTolerations...)
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

func getImagePullPolicy(nfdInstance *nfdv1.NodeFeatureDiscovery) corev1.PullPolicy {
	if nfdInstance.Spec.Operand.ImagePullPolicy != "" {
		return corev1.PullPolicy(nfdInstance.Spec.Operand.ImagePullPolicy)
	}
	return corev1.PullAlways
}

func getArgs(nfdInstance *nfdv1.NodeFeatureDiscovery) []string {
	port := defaultServicePort
	if nfdInstance.Spec.Operand.ServicePort != 0 {
		port = nfdInstance.Spec.Operand.ServicePort
	}
	args := make([]string, 0, 4)
	args = append(args, fmt.Sprintf("--port=%d", port))
	if len(nfdInstance.Spec.ExtraLabelNs) != 0 {
		args = append(args, fmt.Sprintf("--extra-label-ns=%s", strings.Join(nfdInstance.Spec.ExtraLabelNs, ",")))
	}
	if len(nfdInstance.Spec.ResourceLabels) != 0 {
		args = append(args, fmt.Sprintf("--resource-labels=%s", strings.Join(nfdInstance.Spec.ResourceLabels, ",")))
	}

	if strings.TrimSpace(nfdInstance.Spec.LabelWhiteList) != "" {
		args = append(args, fmt.Sprintf("--label-whitelist=%s", nfdInstance.Spec.LabelWhiteList))
	}

	if nfdInstance.Spec.EnableTaints {
		args = append(args, "--enable-taints")
	}

	return args
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

func getMasterSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsNonRoot: ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
		ReadOnlyRootFilesystem: ptr.To(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		AllowPrivilegeEscalation: ptr.To(false),
	}
}

func getGCSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsNonRoot:           ptr.To(true),
		ReadOnlyRootFilesystem: ptr.To(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		AllowPrivilegeEscalation: ptr.To(false),
	}
}

func getLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: 10,
		ProbeHandler: corev1.ProbeHandler{
			GRPC: &corev1.GRPCAction{
				Port: 8082,
			},
		},
	}
}

func getReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: 5,
		FailureThreshold:    10,
		ProbeHandler: corev1.ProbeHandler{
			GRPC: &corev1.GRPCAction{
				Port: 8082,
			},
		},
	}
}
