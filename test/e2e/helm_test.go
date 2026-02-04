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

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	nfdclient "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"

	"sigs.k8s.io/e2e-framework/third_party/helm"
)

// Constants for helm tests
const (
	// nfdNamePrefix is the common prefix used in Helm release resource names
	nfdNamePrefix = "-node-feature-discovery"
	// helmInstallTimeout is the default timeout for Helm install/upgrade operations
	helmInstallTimeout = "5m"
)

// Helper functions for helm tests

// getChartPath returns the absolute path to the local helm chart.
// Tests use the local chart from deployment/helm/node-feature-discovery/
// to verify current changes won't break Helm deployments.
// Uses runtime.Caller to determine path relative to source file location,
// making it independent of the working directory when tests are run.
func getChartPath() string {
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue(), "Failed to get current file path")
	baseDir := filepath.Dir(thisFile)
	return filepath.Clean(filepath.Join(baseDir, "..", "..", "deployment", "helm", "node-feature-discovery"))
}

// getReleaseName generates a unique helm release name using namespace suffix.
// Each test gets a unique namespace (e.g., "node-feature-discovery-12345"),
// so we extract the suffix to create unique release names.
func getReleaseName(prefix, namespace string) string {
	parts := strings.Split(namespace, "-")
	suffix := parts[len(parts)-1]
	return prefix + "-" + suffix
}

// getResourceName returns the full resource name for a Helm-deployed NFD component
func getResourceName(releaseName, suffix string) string {
	return releaseName + nfdNamePrefix + suffix
}

// installHelmChart installs an NFD Helm chart with the given options
func installHelmChart(helmMgr *helm.Manager, releaseName, namespace, chartPath string, extraArgs ...string) error {
	installArgs := []helm.Option{
		helm.WithName(releaseName),
		helm.WithNamespace(namespace),
		helm.WithChart(chartPath),
		helm.WithWait(),
		helm.WithTimeout(helmInstallTimeout),
	}
	if len(extraArgs) > 0 {
		installArgs = append(installArgs, helm.WithArgs(extraArgs...))
	}
	return helmMgr.RunInstall(installArgs...)
}

// uninstallHelmChart uninstalls an NFD Helm release
func uninstallHelmChart(helmMgr *helm.Manager, releaseName, namespace string) error {
	return helmMgr.RunUninstall(
		helm.WithReleaseName(releaseName),
		helm.WithNamespace(namespace),
	)
}

// cleanupHelmRelease performs full cleanup of a Helm release and related resources.
// Errors are logged rather than asserted to avoid masking original test failures.
func cleanupHelmRelease(ctx context.Context, helmMgr *helm.Manager, nfdClient *nfdclient.Clientset,
	cs clientset.Interface, releaseName, namespace string) {
	By("Cleaning up helm release")
	framework.Logf("Helm release %q cleanup", releaseName)
	cleanupNode(ctx, cs)
	cleanupCRs(ctx, nfdClient, namespace)
	err := uninstallHelmChart(helmMgr, releaseName, namespace)
	if err != nil {
		framework.Logf("Warning: failed to uninstall helm release %q: %v", releaseName, err)
	}
}

// Helm E2E test suite
//
// These tests verify that the local Helm chart deploys correctly and creates
// all expected Kubernetes resources. By testing the local chart (not released),
// we can catch regressions before they are merged.
var _ = NFDDescribe(Label("helm"), func() {
	f := framework.NewDefaultFramework("node-feature-discovery")
	// nfd-worker requires privileged access for host mounts
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	//
	// Test basic helm install
	//
	Context("when deploying nfd via local helm chart", func() {
		It("should deploy all components and label nodes", func(ctx context.Context) {
			nfdClient := nfdclient.NewForConfigOrDie(f.ClientConfig())
			helmMgr := helm.New(framework.TestContext.KubeConfig)
			releaseName := getReleaseName("nfd-test", f.Namespace.Name)
			chartPath := getChartPath()

			// Verify chart exists
			_, err := os.Stat(chartPath)
			Expect(err).NotTo(HaveOccurred(), "Local helm chart not found at %s", chartPath)

			By("Installing local nfd helm chart")
			err = installHelmChart(helmMgr, releaseName, f.Namespace.Name, chartPath)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				cleanupHelmRelease(ctx, helmMgr, nfdClient, f.ClientSet, releaseName, f.Namespace.Name)
			}()

			By("Verifying nfd-master deployment exists and is ready")
			deployment, err := f.ClientSet.AppsV1().Deployments(f.Namespace.Name).Get(
				ctx, getResourceName(releaseName, "-master"), metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment.Status.ReadyReplicas).To(Equal(*deployment.Spec.Replicas),
				"nfd-master deployment not fully ready")

			By("Verifying nfd-worker daemonset exists and is running")
			nodes, err := getNonControlPlaneNodes(ctx, f.ClientSet)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				ds, err := f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Get(
					ctx, getResourceName(releaseName, "-worker"), metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ds.Status.NumberReady).To(Equal(int32(len(nodes))),
					"nfd-worker daemonset not running on all nodes")
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			By("Verifying nfd-gc deployment exists and is ready")
			gcDeployment, err := f.ClientSet.AppsV1().Deployments(f.Namespace.Name).Get(
				ctx, getResourceName(releaseName, "-gc"), metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(gcDeployment.Status.ReadyReplicas).To(Equal(*gcDeployment.Spec.Replicas),
				"nfd-gc deployment not fully ready")

			By("Waiting for NFD to discover and label node features")
			Eventually(func(g Gomega) {
				nodes, err := getNonControlPlaneNodes(ctx, f.ClientSet)
				g.Expect(err).NotTo(HaveOccurred())

				for _, node := range nodes {
					hasNFDLabels := false
					for key := range node.Labels {
						if strings.HasPrefix(key, nfdv1alpha1.FeatureLabelNs) {
							hasNFDLabels = true
							break
						}
					}
					g.Expect(hasNFDLabels).To(BeTrue(),
						"Node %q does not have NFD feature labels", node.Name)
				}
			}).WithTimeout(3 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

			By("Verifying NodeFeature CRs are created")
			Eventually(func(g Gomega) {
				nodes, err := getNonControlPlaneNodes(ctx, f.ClientSet)
				g.Expect(err).NotTo(HaveOccurred())

				for _, node := range nodes {
					_, err := nfdClient.NfdV1alpha1().NodeFeatures(f.Namespace.Name).Get(
						ctx, node.Name, metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred(),
						"NodeFeature object not found for node %q", node.Name)
				}
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})
	})

	//
	// Test helm upgrade
	//
	Context("when upgrading the helm release", func() {
		It("should successfully upgrade with modified values", func(ctx context.Context) {
			nfdClient := nfdclient.NewForConfigOrDie(f.ClientConfig())
			helmMgr := helm.New(framework.TestContext.KubeConfig)
			releaseName := getReleaseName("nfd-upgrade", f.Namespace.Name)
			chartPath := getChartPath()

			By("Installing local nfd helm chart for upgrade test")
			err := installHelmChart(helmMgr, releaseName, f.Namespace.Name, chartPath)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				cleanupHelmRelease(ctx, helmMgr, nfdClient, f.ClientSet, releaseName, f.Namespace.Name)
			}()

			By("Upgrading helm release with new replica count")
			err = helmMgr.RunUpgrade(
				helm.WithName(releaseName),
				helm.WithNamespace(f.Namespace.Name),
				helm.WithChart(chartPath),
				helm.WithArgs("--set", "gc.replicaCount=2"),
				helm.WithWait(),
				helm.WithTimeout(helmInstallTimeout),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying nfd-gc deployment has updated replica count")
			Eventually(func(g Gomega) {
				deployment, err := f.ClientSet.AppsV1().Deployments(f.Namespace.Name).Get(
					ctx, getResourceName(releaseName, "-gc"), metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*deployment.Spec.Replicas).To(Equal(int32(2)))
				g.Expect(deployment.Status.ReadyReplicas).To(Equal(int32(2)))
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})
	})

	//
	// Test configuration overrides with table-driven tests
	//
	type configTestCase struct {
		name           string
		helmArgs       []string
		mustExist      []string // Resource suffixes that must exist
		mustNotExist   []string // Resource suffixes that must NOT exist
		mustExistCM    []string // ConfigMap suffixes that must exist
		mustNotExistCM []string // ConfigMap suffixes that must NOT exist
	}

	DescribeTable("configuration overrides",
		func(ctx context.Context, tc configTestCase) {
			nfdClient := nfdclient.NewForConfigOrDie(f.ClientConfig())
			helmMgr := helm.New(framework.TestContext.KubeConfig)
			releaseName := getReleaseName("nfd-config", f.Namespace.Name)
			chartPath := getChartPath()

			By(fmt.Sprintf("Installing helm chart with: %s", tc.name))
			err := installHelmChart(helmMgr, releaseName, f.Namespace.Name, chartPath, tc.helmArgs...)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				cleanupHelmRelease(ctx, helmMgr, nfdClient, f.ClientSet, releaseName, f.Namespace.Name)
			}()

			// Verify resources that must exist
			for _, suffix := range tc.mustExist {
				resourceName := getResourceName(releaseName, suffix)
				By(fmt.Sprintf("Verifying %q exists", resourceName))

				if strings.HasSuffix(suffix, "-master") || strings.HasSuffix(suffix, "-gc") {
					_, err = f.ClientSet.AppsV1().Deployments(f.Namespace.Name).Get(
						ctx, resourceName, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred(), "Deployment %q should exist", resourceName)
				} else {
					_, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Get(
						ctx, resourceName, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred(), "DaemonSet %q should exist", resourceName)
				}
			}

			// Verify resources that must NOT exist
			for _, suffix := range tc.mustNotExist {
				resourceName := getResourceName(releaseName, suffix)
				By(fmt.Sprintf("Verifying %q does NOT exist", resourceName))

				if strings.HasSuffix(suffix, "-gc") {
					_, err = f.ClientSet.AppsV1().Deployments(f.Namespace.Name).Get(
						ctx, resourceName, metav1.GetOptions{})
					Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Deployment %q should not exist", resourceName)
				} else {
					_, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Get(
						ctx, resourceName, metav1.GetOptions{})
					Expect(apierrors.IsNotFound(err)).To(BeTrue(), "DaemonSet %q should not exist", resourceName)
				}
			}

			// Verify ConfigMaps that must exist
			for _, suffix := range tc.mustExistCM {
				cmName := getResourceName(releaseName, suffix)
				By(fmt.Sprintf("Verifying ConfigMap %q exists", cmName))
				_, err = f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Get(
					ctx, cmName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "ConfigMap %q should exist", cmName)
			}

			// Verify ConfigMaps that must NOT exist
			for _, suffix := range tc.mustNotExistCM {
				cmName := getResourceName(releaseName, suffix)
				By(fmt.Sprintf("Verifying ConfigMap %q does NOT exist", cmName))
				_, err = f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Get(
					ctx, cmName, metav1.GetOptions{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "ConfigMap %q should not exist", cmName)
			}

			// Special check for topology-updater label when it exists
			if slices.Contains(tc.mustExist, "-topology-updater") {
				By("Verifying topology-updater has correct labels")
				// Note: We only verify the DaemonSet is created by the Helm chart.
				// Pods won't become ready on Kind clusters because topology-updater
				// requires NUMA node information (/sys/bus/node/devices) which Docker
				// containers don't have. Full functionality is tested in
				// topology_updater_test.go on clusters with proper NUMA topology.
				ds, err := f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Get(
					ctx, getResourceName(releaseName, "-topology-updater"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(ds.Spec.Selector.MatchLabels["app.kubernetes.io/name"]).To(Equal("node-feature-discovery"))
			}
		},
		Entry("topology-updater disabled", configTestCase{
			name:     "topology-updater disabled",
			helmArgs: []string{"--set", "topologyUpdater.enable=false"},
			mustExist: []string{
				"-master",
				"-worker",
				"-gc",
			},
			mustNotExist: []string{
				"-topology-updater",
			},
			mustExistCM: []string{
				"-master-conf",
				"-worker-conf",
			},
			mustNotExistCM: []string{
				"-topology-updater-conf",
			},
		}),
		Entry("topology-updater enabled", configTestCase{
			name:     "topology-updater enabled",
			helmArgs: []string{"--set", "topologyUpdater.enable=true"},
			mustExist: []string{
				"-master",
				"-worker",
				"-gc",
				"-topology-updater",
			},
			mustNotExist: []string{},
			mustExistCM: []string{
				"-master-conf",
				"-worker-conf",
				"-topology-updater-conf",
			},
			mustNotExistCM: []string{},
		}),
		Entry("gc disabled", configTestCase{
			name:     "gc disabled",
			helmArgs: []string{"--set", "gc.enable=false"},
			mustExist: []string{
				"-master",
				"-worker",
			},
			mustNotExist: []string{
				"-gc",
			},
			mustExistCM: []string{
				"-master-conf",
				"-worker-conf",
			},
			mustNotExistCM: []string{},
		}),
	)

	//
	// Test RBAC and configuration resources with table-driven tests
	//
	type rbacTestCase struct {
		name                   string
		helmArgs               []string
		serviceAccounts        []string // ServiceAccount suffixes that must exist
		missingServiceAccounts []string // ServiceAccount suffixes that must NOT exist
		configMaps             []string // ConfigMap suffixes that must exist
		missingConfigMaps      []string // ConfigMap suffixes that must NOT exist
		roles                  []string // Role suffixes (namespaced)
		clusterRoles           []string // ClusterRole suffixes
		missingClusterRoles    []string // ClusterRole suffixes that must NOT exist
	}

	DescribeTable("RBAC and configuration resources",
		func(ctx context.Context, tc rbacTestCase) {
			nfdClient := nfdclient.NewForConfigOrDie(f.ClientConfig())
			helmMgr := helm.New(framework.TestContext.KubeConfig)
			releaseName := getReleaseName("nfd-rbac", f.Namespace.Name)
			chartPath := getChartPath()

			By(fmt.Sprintf("Installing helm chart with: %s", tc.name))
			err := installHelmChart(helmMgr, releaseName, f.Namespace.Name, chartPath, tc.helmArgs...)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				cleanupHelmRelease(ctx, helmMgr, nfdClient, f.ClientSet, releaseName, f.Namespace.Name)
			}()

			// Verify ServiceAccounts that must exist
			for _, suffix := range tc.serviceAccounts {
				saName := getResourceName(releaseName, suffix)
				By(fmt.Sprintf("Verifying ServiceAccount %q exists", saName))
				_, err := f.ClientSet.CoreV1().ServiceAccounts(f.Namespace.Name).Get(
					ctx, saName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "ServiceAccount %q should exist", saName)
			}

			// Verify ServiceAccounts that must NOT exist
			for _, suffix := range tc.missingServiceAccounts {
				saName := getResourceName(releaseName, suffix)
				By(fmt.Sprintf("Verifying ServiceAccount %q does NOT exist", saName))
				_, err := f.ClientSet.CoreV1().ServiceAccounts(f.Namespace.Name).Get(
					ctx, saName, metav1.GetOptions{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "ServiceAccount %q should not exist", saName)
			}

			// Verify ConfigMaps that must exist
			for _, suffix := range tc.configMaps {
				cmName := getResourceName(releaseName, suffix)
				By(fmt.Sprintf("Verifying ConfigMap %q exists", cmName))
				_, err := f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Get(
					ctx, cmName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "ConfigMap %q should exist", cmName)
			}

			// Verify ConfigMaps that must NOT exist
			for _, suffix := range tc.missingConfigMaps {
				cmName := getResourceName(releaseName, suffix)
				By(fmt.Sprintf("Verifying ConfigMap %q does NOT exist", cmName))
				_, err := f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Get(
					ctx, cmName, metav1.GetOptions{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "ConfigMap %q should not exist", cmName)
			}

			// Verify Roles (namespaced RBAC)
			for _, suffix := range tc.roles {
				roleName := getResourceName(releaseName, suffix)
				By(fmt.Sprintf("Verifying Role %q exists", roleName))
				_, err := f.ClientSet.RbacV1().Roles(f.Namespace.Name).Get(
					ctx, roleName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "Role %q should exist", roleName)

				// Also verify corresponding RoleBinding
				By(fmt.Sprintf("Verifying RoleBinding %q exists", roleName))
				_, err = f.ClientSet.RbacV1().RoleBindings(f.Namespace.Name).Get(
					ctx, roleName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "RoleBinding %q should exist", roleName)
			}

			// Verify ClusterRoles that must exist
			for _, suffix := range tc.clusterRoles {
				crName := getResourceName(releaseName, suffix)
				By(fmt.Sprintf("Verifying ClusterRole %q exists", crName))
				_, err := f.ClientSet.RbacV1().ClusterRoles().Get(
					ctx, crName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "ClusterRole %q should exist", crName)

				// Also verify corresponding ClusterRoleBinding
				By(fmt.Sprintf("Verifying ClusterRoleBinding %q exists", crName))
				_, err = f.ClientSet.RbacV1().ClusterRoleBindings().Get(
					ctx, crName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "ClusterRoleBinding %q should exist", crName)
			}

			// Verify ClusterRoles that must NOT exist
			for _, suffix := range tc.missingClusterRoles {
				crName := getResourceName(releaseName, suffix)
				By(fmt.Sprintf("Verifying ClusterRole %q does NOT exist", crName))
				_, err := f.ClientSet.RbacV1().ClusterRoles().Get(
					ctx, crName, metav1.GetOptions{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "ClusterRole %q should not exist", crName)

				// Also verify corresponding ClusterRoleBinding doesn't exist
				By(fmt.Sprintf("Verifying ClusterRoleBinding %q does NOT exist", crName))
				_, err = f.ClientSet.RbacV1().ClusterRoleBindings().Get(
					ctx, crName, metav1.GetOptions{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "ClusterRoleBinding %q should not exist", crName)
			}
		},
		Entry("default RBAC resources", rbacTestCase{
			name:     "default RBAC resources",
			helmArgs: []string{},
			serviceAccounts: []string{
				"",
				"-gc",
				"-worker",
			},
			configMaps: []string{
				"-master-conf",
				"-worker-conf",
			},
			roles: []string{
				"-worker",
			},
			clusterRoles: []string{
				"",
				"-gc",
			},
		}),
		Entry("topology-updater RBAC when enabled", rbacTestCase{
			name:     "topology-updater RBAC when enabled",
			helmArgs: []string{"--set", "topologyUpdater.enable=true"},
			serviceAccounts: []string{
				"",
				"-gc",
				"-worker",
				"-topology-updater",
			},
			configMaps: []string{
				"-master-conf",
				"-worker-conf",
				"-topology-updater-conf",
			},
			roles: []string{
				"-worker",
			},
			clusterRoles: []string{
				"",
				"-gc",
				"-topology-updater",
			},
		}),
		Entry("gc disabled removes gc RBAC", rbacTestCase{
			name:     "gc disabled removes gc RBAC",
			helmArgs: []string{"--set", "gc.enable=false"},
			serviceAccounts: []string{
				"",
				"-worker",
			},
			missingServiceAccounts: []string{
				"-gc",
			},
			configMaps: []string{
				"-master-conf",
				"-worker-conf",
			},
			roles: []string{
				"-worker",
			},
			clusterRoles: []string{
				"",
			},
			missingClusterRoles: []string{
				"-gc",
			},
		}),
	)

	//
	// Test helm uninstall and cleanup
	//
	Context("when uninstalling the helm release", func() {
		It("should remove all NFD resources", func(ctx context.Context) {
			helmMgr := helm.New(framework.TestContext.KubeConfig)
			releaseName := getReleaseName("nfd-uninstall", f.Namespace.Name)
			chartPath := getChartPath()

			By("Installing local nfd helm chart for cleanup test")
			err := installHelmChart(helmMgr, releaseName, f.Namespace.Name, chartPath)
			Expect(err).NotTo(HaveOccurred())

			// Wait for deployment to be ready
			Eventually(func(g Gomega) {
				deployment, err := f.ClientSet.AppsV1().Deployments(f.Namespace.Name).Get(
					ctx, getResourceName(releaseName, "-master"), metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deployment.Status.ReadyReplicas).To(Equal(*deployment.Spec.Replicas))
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			By("Uninstalling the helm release")
			err = uninstallHelmChart(helmMgr, releaseName, f.Namespace.Name)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying deployments are removed")
			Eventually(func(g Gomega) {
				deployments, err := f.ClientSet.AppsV1().Deployments(f.Namespace.Name).List(
					ctx, metav1.ListOptions{})
				g.Expect(err).NotTo(HaveOccurred())

				for _, dep := range deployments.Items {
					g.Expect(strings.HasPrefix(dep.Name, releaseName)).To(BeFalse(),
						"Deployment %q should have been removed", dep.Name)
				}
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			By("Verifying daemonsets are removed")
			Eventually(func(g Gomega) {
				daemonsets, err := f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).List(
					ctx, metav1.ListOptions{})
				g.Expect(err).NotTo(HaveOccurred())

				for _, ds := range daemonsets.Items {
					g.Expect(strings.HasPrefix(ds.Name, releaseName)).To(BeFalse(),
						"DaemonSet %q should have been removed", ds.Name)
				}
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})
	})
})
