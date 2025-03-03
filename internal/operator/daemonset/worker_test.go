package daemonset

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
)

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
