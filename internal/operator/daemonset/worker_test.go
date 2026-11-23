package daemonset

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	nfdv1 "sigs.k8s.io/node-feature-discovery/api/operator/v1"
	"sigs.k8s.io/yaml"
)

// nfdFromYAML decodes a NodeFeatureDiscovery the way the operator receives
// it, so a test can set fields that the Go type may not carry.
func nfdFromYAML(doc string) *nfdv1.NodeFeatureDiscovery {
	nfd := &nfdv1.NodeFeatureDiscovery{}
	ExpectWithOffset(1, yaml.Unmarshal([]byte(doc), nfd)).To(Succeed())
	return nfd
}

// desiredWorkerContainer runs SetWorkerDaemonsetAsDesired and returns the
// worker container it produced.
func desiredWorkerContainer(nfd *nfdv1.NodeFeatureDiscovery) (corev1.Container, error) {
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "nfd-worker", Namespace: "test-namespace"},
	}
	err := NewDaemonsetAPI(nil, scheme).SetWorkerDaemonsetAsDesired(context.Background(), nfd, &ds)
	if err != nil {
		return corev1.Container{}, err
	}
	ExpectWithOffset(1, ds.Spec.Template.Spec.Containers).To(HaveLen(1))
	return ds.Spec.Template.Spec.Containers[0], nil
}

// hardenedWorkerSecurityContext is the worker security context the operator
// set before spec.operand.workerSecurityContext existed.
func hardenedWorkerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		ReadOnlyRootFilesystem:   ptr.To(true),
		RunAsNonRoot:             ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}
}

var _ = Describe("worker container securityContext", func() {

	DescribeTable("without an override, the hardened context is unchanged",
		func(doc string) {
			c, err := desiredWorkerContainer(nfdFromYAML(doc))
			Expect(err).NotTo(HaveOccurred())
			Expect(c.SecurityContext).To(Equal(hardenedWorkerSecurityContext()))
		},
		Entry("workerSecurityContext absent", `
spec:
  operand:
    image: test-image
`),
		Entry("workerSecurityContext empty", `
spec:
  operand:
    image: test-image
    workerSecurityContext: {}
`),
	)

	It("adds a Localhost AppArmor profile from the override", func() {
		c, err := desiredWorkerContainer(nfdFromYAML(`
spec:
  operand:
    image: test-image
    workerSecurityContext:
      appArmorProfile:
        type: Localhost
        localhostProfile: nfd-worker
`))
		Expect(err).NotTo(HaveOccurred())

		expected := hardenedWorkerSecurityContext()
		expected.AppArmorProfile = &corev1.AppArmorProfile{
			Type:             corev1.AppArmorProfileTypeLocalhost,
			LocalhostProfile: ptr.To("nfd-worker"),
		}
		Expect(c.SecurityContext).To(Equal(expected))
	})

	It("adds SELinux options from the override", func() {
		c, err := desiredWorkerContainer(nfdFromYAML(`
spec:
  operand:
    image: test-image
    workerSecurityContext:
      seLinuxOptions:
        type: nfd_worker_t
        level: s0
`))
		Expect(err).NotTo(HaveOccurred())

		expected := hardenedWorkerSecurityContext()
		expected.SELinuxOptions = &corev1.SELinuxOptions{Type: "nfd_worker_t", Level: "s0"}
		Expect(c.SecurityContext).To(Equal(expected))
	})

	It("does not let the override weaken the hardened fields", func() {
		c, err := desiredWorkerContainer(nfdFromYAML(`
spec:
  operand:
    image: test-image
    workerSecurityContext:
      privileged: true
      allowPrivilegeEscalation: true
      readOnlyRootFilesystem: false
      runAsNonRoot: false
      runAsUser: 0
      procMount: Unmasked
      capabilities:
        add: ["SYS_ADMIN"]
        drop: []
      seccompProfile:
        type: Unconfined
      appArmorProfile:
        type: Localhost
        localhostProfile: nfd-worker
`))
		Expect(err).NotTo(HaveOccurred())

		Expect(c.SecurityContext.Privileged).To(BeNil())
		Expect(c.SecurityContext.AllowPrivilegeEscalation).To(Equal(ptr.To(false)))
		expected := hardenedWorkerSecurityContext()
		expected.AppArmorProfile = &corev1.AppArmorProfile{
			Type:             corev1.AppArmorProfileTypeLocalhost,
			LocalhostProfile: ptr.To("nfd-worker"),
		}
		Expect(c.SecurityContext).To(Equal(expected))
	})

	DescribeTable("rejects an override that turns confinement off",
		func(doc, msg string) {
			_, err := desiredWorkerContainer(nfdFromYAML(doc))
			Expect(err).To(MatchError(msg))
		},
		Entry("AppArmor Unconfined", `
spec:
  operand:
    workerSecurityContext:
      appArmorProfile:
        type: Unconfined
`, `spec.operand.workerSecurityContext.appArmorProfile.type "Unconfined" is not allowed: nfd-worker must run confined`),
		Entry("SELinux spc_t", `
spec:
  operand:
    workerSecurityContext:
      seLinuxOptions:
        type: spc_t
`, `spec.operand.workerSecurityContext.seLinuxOptions.type "spc_t" is not allowed: nfd-worker must run confined`),
		Entry("SELinux unconfined_t", `
spec:
  operand:
    workerSecurityContext:
      seLinuxOptions:
        type: unconfined_t
`, `spec.operand.workerSecurityContext.seLinuxOptions.type "unconfined_t" is not allowed: nfd-worker must run confined`),
	)
})

var _ = Describe("getWorkerToleration", func() {

	It("worker tolerations are not defined in NFD CR", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				Operand: nfdv1.OperandSpec{},
			},
		}
		expectedTolerations := []corev1.Toleration{
			{
				Operator: "Exists",
				Effect:   "NoSchedule",
			},
		}

		res := getWorkerTolerations(&nfdCR)
		Expect(res).To(Equal(expectedTolerations))
	})

	It("worker tolerations are defined in NFD CR", func() {
		workerTolerations := []corev1.Toleration{
			{
				Key:      "key1",
				Value:    "value1",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "key1",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		}
		nfdCR := nfdv1.NodeFeatureDiscovery{
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				Operand: nfdv1.OperandSpec{
					WorkerTolerations: workerTolerations,
				},
			},
		}
		expectedTolerations := []corev1.Toleration{
			{
				Operator: "Exists",
				Effect:   "NoSchedule",
			},
			{
				Key:      "key1",
				Value:    "value1",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "key1",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		}

		res := getWorkerTolerations(&nfdCR)
		Expect(res).To(Equal(expectedTolerations))
	})
})

var _ = Describe("worker host mounts", func() {

	DescribeTable("are read-only",
		func(mountPath string) {
			c, err := desiredWorkerContainer(nfdFromYAML(`
spec:
  operand:
    image: test-image
`))
			Expect(err).NotTo(HaveOccurred())

			readOnly := map[string]bool{}
			for _, m := range c.VolumeMounts {
				readOnly[m.MountPath] = m.ReadOnly
			}
			Expect(readOnly).To(HaveKeyWithValue(mountPath, true))
		},
		Entry("source.d", "/etc/kubernetes/node-feature-discovery/source.d"),
		Entry("features.d", "/etc/kubernetes/node-feature-discovery/features.d"),
	)
})
