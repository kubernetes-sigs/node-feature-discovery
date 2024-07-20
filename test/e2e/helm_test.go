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
package e2e

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

// Helm based test suite
var _ = NFDDescribe(Label("helm"), func() {
	f := framework.NewDefaultFramework("node-feature-discovery")
	// To avoid the error of violating the PodSecurity
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	manager := helm.New(framework.TestContext.KubeConfig)
	release_name := "node-feature-discovery"

	Context("when deploying by helm", Ordered, func() {
		JustBeforeEach(func(ctx context.Context) {
			// Install local nfd chart
			workingDir, err := os.Getwd()
			chartPath := filepath.Join(workingDir, "../../", "deployment", "helm", "node-feature-discovery")
			Expect(err).NotTo(HaveOccurred())
			By("Installing nfd helm chart")
			err = manager.RunInstall(
				helm.WithName(release_name),
				helm.WithNamespace(f.Namespace.Name),
				helm.WithChart(chartPath),
				helm.WithWait(),
			)
			Expect(err).NotTo(HaveOccurred())

			// Show all pods
			pods, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).List(ctx, v1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods.Items {
				By("Pod name: " + pod.Name)
			}
			// Get the name of the master pod
			masterPods, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).List(ctx, v1.ListOptions{LabelSelector: "role=master"})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(masterPods.Items)).To(Equal(1))
			// Wait for the master pod to be running
			Expect(e2epod.WaitTimeoutForPodRunningInNamespace(ctx, f.ClientSet, masterPods.Items[0].Name, f.Namespace.Name, time.Minute)).NotTo(HaveOccurred())
		})

		AfterEach(func(ctx context.Context) {
			// Uninstall nfd
			By("Uninstalling nfd helm chart")
			err := manager.RunUninstall(
				helm.WithReleaseName(release_name),
				helm.WithNamespace(f.Namespace.Name),
			)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("and nfd deployed by Helm", func() {
			It("Deployment is running successfully", Label("Helm"), func(ctx context.Context) {
			})

			It("Deployment with helm again is running successfully", Label("Helm"), func(ctx context.Context) {
			})
		})
	})
})
