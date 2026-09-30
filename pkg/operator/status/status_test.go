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

package status

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
	"sigs.k8s.io/node-feature-discovery-operator/internal/daemonset"
	"sigs.k8s.io/node-feature-discovery-operator/internal/deployment"
)

var _ = Describe("GetConditions", func() {
	var (
		ctrl       *gomock.Controller
		mockHelper *MockstatusHelperAPI
		st         *status
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockHelper = NewMockstatusHelperAPI(ctrl)
		st = &status{
			helper: mockHelper,
		}
	})

	ctx := context.Background()
	nfdCR := nfdv1.NodeFeatureDiscovery{
		Spec: nfdv1.NodeFeatureDiscoverySpec{
			TopologyUpdater: true,
		},
	}
	progConds := getProgressingConditions("progressing reason", "progressing message")
	degConds := getDegradedConditions("degraded reason", "degraded message")
	availConds := getAvailableConditions()

	DescribeTable("checking all the flows", func(workerAvailable, masterAvailable, gcAvailable, topologyAvailable bool) {
		expectConds := availConds
		if !workerAvailable {
			mockHelper.EXPECT().getWorkerNotAvailableConditions(ctx, &nfdCR).Return(degConds)
			expectConds = degConds
			goto executeTestFunction
		}
		mockHelper.EXPECT().getWorkerNotAvailableConditions(ctx, &nfdCR).Return(nil)
		if !masterAvailable {
			mockHelper.EXPECT().getMasterNotAvailableConditions(ctx, &nfdCR).Return(progConds)
			expectConds = progConds
			goto executeTestFunction
		}
		mockHelper.EXPECT().getMasterNotAvailableConditions(ctx, &nfdCR).Return(nil)
		if !gcAvailable {
			mockHelper.EXPECT().getGCNotAvailableConditions(ctx, &nfdCR).Return(degConds)
			expectConds = degConds
			goto executeTestFunction
		}
		mockHelper.EXPECT().getGCNotAvailableConditions(ctx, &nfdCR).Return(nil)
		if !topologyAvailable {
			mockHelper.EXPECT().getTopologyNotAvailableConditions(ctx, &nfdCR).Return(progConds)
			expectConds = progConds
		} else {
			mockHelper.EXPECT().getTopologyNotAvailableConditions(ctx, &nfdCR).Return(nil)
		}

	executeTestFunction:
		conds := st.GetConditions(ctx, &nfdCR)
		compareConditions(conds, expectConds)
	},
		Entry("worker is not available yet", false, false, false, false),
		Entry("worker available, master is not yet", true, false, false, false),
		Entry("worker and master available, gc is not yet", true, true, false, false),
		Entry("worker,master and gc available, topology is not yet", true, true, true, false),
		Entry("all components are available", true, true, true, true),
	)
})

var _ = Describe("AreConditionsEqual", func() {
	It("testing various use-cases", func() {
		st := &status{}

		By("progressing conditions, reason not equal")
		firstCond := getProgressingConditions("reason1", "message1")
		secondCond := getProgressingConditions("reason2", "message1")
		res := st.AreConditionsEqual(firstCond, secondCond)
		Expect(res).To(BeFalse())

		By("progressing conditions, message not equal")
		firstCond = getProgressingConditions("reason1", "message1")
		secondCond = getProgressingConditions("reason1", "message2")
		res = st.AreConditionsEqual(firstCond, secondCond)
		Expect(res).To(BeFalse())

		By("progressing conditions equal")
		firstCond = getProgressingConditions("reason1", "message1")
		secondCond = getProgressingConditions("reason1", "message1")
		res = st.AreConditionsEqual(firstCond, secondCond)
		Expect(res).To(BeTrue())

		By("degraded conditions, reason not equal")
		firstCond = getDegradedConditions("reason1", "message1")
		secondCond = getDegradedConditions("reason2", "message1")
		res = st.AreConditionsEqual(firstCond, secondCond)
		Expect(res).To(BeFalse())

		By("degraded conditions, message not equal")
		firstCond = getDegradedConditions("reason1", "message1")
		secondCond = getDegradedConditions("reason1", "message2")
		res = st.AreConditionsEqual(firstCond, secondCond)
		Expect(res).To(BeFalse())

		By("degraded conditions equal")
		firstCond = getDegradedConditions("reason1", "message1")
		secondCond = getDegradedConditions("reason1", "message1")
		res = st.AreConditionsEqual(firstCond, secondCond)
		Expect(res).To(BeTrue())

		By("available conditions equal")
		firstCond = getAvailableConditions()
		secondCond = getAvailableConditions()
		res = st.AreConditionsEqual(firstCond, secondCond)
		Expect(res).To(BeTrue())

		By("degraded and progressing conditions are not equal")
		firstCond = getDegradedConditions("reason1", "message1")
		secondCond = getProgressingConditions("reason1", "message1")
		res = st.AreConditionsEqual(firstCond, secondCond)
		Expect(res).To(BeFalse())
	})
})

var _ = Describe("getWorkerOrTopologyNotAvailableConditions", func() {
	var (
		ctrl   *gomock.Controller
		mockDS *daemonset.MockDaemonsetAPI
		h      statusHelperAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockDS = daemonset.NewMockDaemonsetAPI(ctrl)
		h = newStatusHelperAPI(nil, mockDS)
	})

	nfdCR := nfdv1.NodeFeatureDiscovery{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
		},
	}

	ctx := context.Background()

	It("worker of topology ds not available", func() {
		err := fmt.Errorf("some error")
		By("checking worker")
		expectedConds := getDegradedConditions(conditionFailedGettingNFDWorkerDaemonSet, err.Error())
		mockDS.EXPECT().GetDaemonSet(ctx, nfdCR.Namespace, "nfd-worker").Return(nil, err)

		resCond := h.getWorkerNotAvailableConditions(ctx, &nfdCR)
		compareConditions(resCond, expectedConds)

		By("checking topology")
		expectedConds = getDegradedConditions(conditionFailedGettingNFDTopologyDaemonSet, err.Error())
		mockDS.EXPECT().GetDaemonSet(ctx, nfdCR.Namespace, "nfd-topology-updater").Return(nil, err)

		resCond = h.getTopologyNotAvailableConditions(ctx, &nfdCR)
		compareConditions(resCond, expectedConds)
	})

	It("worker or topology ds number of scheduled is 0", func() {
		ds := &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 0,
			},
		}
		By("checking worker")
		expectedConds := getDegradedConditions(conditionNFDWorkerDaemonSetDegraded, "number of desired nodes for scheduling is 0")
		mockDS.EXPECT().GetDaemonSet(ctx, nfdCR.Namespace, "nfd-worker").Return(ds, nil)

		resCond := h.getWorkerNotAvailableConditions(ctx, &nfdCR)
		compareConditions(resCond, expectedConds)

		By("checking topology")
		expectedConds = getDegradedConditions(conditionNFDTopologyDaemonSetDegraded, "number of desired nodes for scheduling is 0")
		mockDS.EXPECT().GetDaemonSet(ctx, nfdCR.Namespace, "nfd-topology-updater").Return(ds, nil)

		resCond = h.getTopologyNotAvailableConditions(ctx, &nfdCR)
		compareConditions(resCond, expectedConds)
	})

	It("worker or topology ds current number of scheduled pods is 0", func() {
		ds := &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 2,
				CurrentNumberScheduled: 0,
			},
		}
		By("checking worker")
		expectedConds := getDegradedConditions(conditionNFDWorkerDaemonSetDegraded, "0 nodes have pods scheduled")
		mockDS.EXPECT().GetDaemonSet(ctx, nfdCR.Namespace, "nfd-worker").Return(ds, nil)

		resCond := h.getWorkerNotAvailableConditions(ctx, &nfdCR)
		compareConditions(resCond, expectedConds)

		By("checking topology")
		expectedConds = getDegradedConditions(conditionNFDTopologyDaemonSetDegraded, "0 nodes have pods scheduled")
		mockDS.EXPECT().GetDaemonSet(ctx, nfdCR.Namespace, "nfd-topology-updater").Return(ds, nil)

		resCond = h.getTopologyNotAvailableConditions(ctx, &nfdCR)
		compareConditions(resCond, expectedConds)
	})

	It("worker or topology ds number of pods has not yet reached desired number", func() {
		ds := &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 2,
				CurrentNumberScheduled: 2,
				NumberReady:            1,
			},
		}
		By("worker")
		expectedConds := getProgressingConditions(conditionNFDWorkerDaemonSetProgressing, "ds is progressing")
		mockDS.EXPECT().GetDaemonSet(ctx, nfdCR.Namespace, "nfd-worker").Return(ds, nil)

		resCond := h.getWorkerNotAvailableConditions(ctx, &nfdCR)
		compareConditions(resCond, expectedConds)

		By("topology")
		expectedConds = getProgressingConditions(conditionNFDTopologyDaemonSetProgressing, "ds is progressing")
		mockDS.EXPECT().GetDaemonSet(ctx, nfdCR.Namespace, "nfd-topology-updater").Return(ds, nil)

		resCond = h.getTopologyNotAvailableConditions(ctx, &nfdCR)
		compareConditions(resCond, expectedConds)
	})

	It("worker or topology ds all pods are available", func() {
		ds := &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 2,
				CurrentNumberScheduled: 2,
				NumberReady:            2,
			},
		}
		By("worker")
		mockDS.EXPECT().GetDaemonSet(ctx, nfdCR.Namespace, "nfd-worker").Return(ds, nil)

		resCond := h.getWorkerNotAvailableConditions(ctx, &nfdCR)
		Expect(resCond).To(BeNil())

		By("topology")
		mockDS.EXPECT().GetDaemonSet(ctx, nfdCR.Namespace, "nfd-topology-updater").Return(ds, nil)

		resCond = h.getTopologyNotAvailableConditions(ctx, &nfdCR)
		Expect(resCond).To(BeNil())
	})
})

var _ = Describe("getMasterOrGCNotAvailableCondition", func() {
	var (
		ctrl           *gomock.Controller
		mockDeployment *deployment.MockDeploymentAPI
		h              statusHelperAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockDeployment = deployment.NewMockDeploymentAPI(ctrl)
		h = newStatusHelperAPI(mockDeployment, nil)
	})

	nfdCR := nfdv1.NodeFeatureDiscovery{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
		},
	}
	ctx := context.Background()

	It("master or GC deployment not available", func() {
		err := fmt.Errorf("some error")

		By("master")
		expectedConds := getDegradedConditions(conditionFailedGettingNFDMasterDeployment, err.Error())
		mockDeployment.EXPECT().GetDeployment(ctx, nfdCR.Namespace, "nfd-master").Return(nil, err)

		resCond := h.getMasterNotAvailableConditions(ctx, &nfdCR)
		compareConditions(resCond, expectedConds)

		By("GC")
		expectedConds = getDegradedConditions(conditionFailedGettingNFDGCDeployment, err.Error())
		mockDeployment.EXPECT().GetDeployment(ctx, nfdCR.Namespace, "nfd-gc").Return(nil, err)

		resCond = h.getGCNotAvailableConditions(ctx, &nfdCR)
		compareConditions(resCond, expectedConds)
	})

	It("master or GC deployment available replicas 0", func() {
		dep := &appsv1.Deployment{
			Status: appsv1.DeploymentStatus{
				AvailableReplicas: 0,
			},
		}
		By("master")
		expectedConds := getDegradedConditions(conditionNFDMasterDeploymentDegraded, "number of available pods is 0")
		mockDeployment.EXPECT().GetDeployment(ctx, nfdCR.Namespace, "nfd-master").Return(dep, nil)

		resCond := h.getMasterNotAvailableConditions(ctx, &nfdCR)
		compareConditions(resCond, expectedConds)

		By("GC")
		expectedConds = getDegradedConditions(conditionNFDGCDeploymentDegraded, "number of available pods is 0")
		mockDeployment.EXPECT().GetDeployment(ctx, nfdCR.Namespace, "nfd-gc").Return(dep, nil)

		resCond = h.getGCNotAvailableConditions(ctx, &nfdCR)
		compareConditions(resCond, expectedConds)
	})

	It("master or GC deployment all pods are available", func() {
		dep := &appsv1.Deployment{
			Status: appsv1.DeploymentStatus{
				AvailableReplicas: 1,
			},
		}
		By("master")
		mockDeployment.EXPECT().GetDeployment(ctx, nfdCR.Namespace, "nfd-master").Return(dep, nil)

		resCond := h.getMasterNotAvailableConditions(ctx, &nfdCR)
		Expect(resCond).To(BeNil())

		By("GC")
		mockDeployment.EXPECT().GetDeployment(ctx, nfdCR.Namespace, "nfd-gc").Return(dep, nil)

		resCond = h.getGCNotAvailableConditions(ctx, &nfdCR)
		Expect(resCond).To(BeNil())
	})
})

func compareConditions(first, second []metav1.Condition) {
	Expect(len(first)).To(Equal(len(second)))
	testTimestamp := metav1.Time{Time: time.Now()}
	for i := 0; i < len(first); i++ {
		first[i].LastTransitionTime = testTimestamp
		second[i].LastTransitionTime = testTimestamp
	}
	Expect(first).To(Equal(second))
}
