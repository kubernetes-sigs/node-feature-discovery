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

package new_controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"

	"sigs.k8s.io/node-feature-discovery-operator/internal/client"
	"sigs.k8s.io/node-feature-discovery-operator/internal/configmap"
	"sigs.k8s.io/node-feature-discovery-operator/internal/daemonset"
	"sigs.k8s.io/node-feature-discovery-operator/internal/deployment"
)

var _ = Describe("Reconcile", func() {
	var (
		ctrl       *gomock.Controller
		mockHelper *MocknodeFeatureDiscoveryHelperAPI
		nfdr       *nodeFeatureDiscoveryReconciler
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockHelper = NewMocknodeFeatureDiscoveryHelperAPI(ctrl)

		nfdr = &nodeFeatureDiscoveryReconciler{
			helper: mockHelper,
		}
	})

	ctx := context.Background()

	It("good flow without finalization", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{}

		mockHelper.EXPECT().hasFinalizer(&nfdCR).Return(true)
		mockHelper.EXPECT().handleMaster(ctx, &nfdCR).Return(nil)
		mockHelper.EXPECT().handleWorker(ctx, &nfdCR).Return(nil)
		mockHelper.EXPECT().handleTopology(ctx, &nfdCR).Return(nil)
		mockHelper.EXPECT().handleGC(ctx, &nfdCR).Return(nil)
		mockHelper.EXPECT().handlePrune(ctx, &nfdCR).Return(nil)
		mockHelper.EXPECT().handleStatus(ctx, &nfdCR).Return(nil)

		res, err := nfdr.Reconcile(ctx, &nfdCR)
		Expect(res).To(Equal(reconcile.Result{}))
		Expect(err).To(BeNil())
	})

	DescribeTable("finalization flow", func(finalizationError error) {
		nfdCR := nfdv1.NodeFeatureDiscovery{}
		timestamp := metav1.Now()
		nfdCR.SetDeletionTimestamp(&timestamp)
		mockHelper.EXPECT().finalizeComponents(ctx, &nfdCR).Return(finalizationError)

		res, err := nfdr.Reconcile(ctx, &nfdCR)
		Expect(res).To(Equal(reconcile.Result{}))
		if finalizationError != nil {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).To(BeNil())
		}
	},
		Entry("finalization failed", fmt.Errorf("finalization error")),
		Entry("finalization succeeded", fmt.Errorf("finalization error")),
	)

	DescribeTable("setFinalizer flow", func(setFinalizerError error) {
		nfdCR := nfdv1.NodeFeatureDiscovery{}
		mockHelper.EXPECT().hasFinalizer(&nfdCR).Return(false)
		mockHelper.EXPECT().setFinalizer(ctx, &nfdCR).Return(setFinalizerError)

		res, err := nfdr.Reconcile(ctx, &nfdCR)
		Expect(res).To(Equal(reconcile.Result{}))
		if setFinalizerError != nil {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).To(BeNil())
		}
	},
		Entry("setFinalizer failed", fmt.Errorf("set finalizer error")),
		Entry("setFinalizer succeeded", fmt.Errorf("set finalizer error")),
	)

	DescribeTable("check components error flows", func(handlerMasterError,
		handlerWorkerError,
		handleTopologyError,
		handlerGCError,
		handlePruneError,
		handleStatusError error) {
		nfdCR := nfdv1.NodeFeatureDiscovery{}

		mockHelper.EXPECT().hasFinalizer(&nfdCR).Return(true)
		mockHelper.EXPECT().handleMaster(ctx, &nfdCR).Return(handlerMasterError)
		mockHelper.EXPECT().handleWorker(ctx, &nfdCR).Return(handlerWorkerError)
		mockHelper.EXPECT().handleTopology(ctx, &nfdCR).Return(handleTopologyError)
		mockHelper.EXPECT().handleGC(ctx, &nfdCR).Return(handlerGCError)
		mockHelper.EXPECT().handlePrune(ctx, &nfdCR).Return(handlePruneError)
		mockHelper.EXPECT().handleStatus(ctx, &nfdCR).Return(handleStatusError)

		res, err := nfdr.Reconcile(ctx, &nfdCR)
		Expect(res).To(Equal(reconcile.Result{}))
		if handlerMasterError != nil || handlerWorkerError != nil || handleTopologyError != nil ||
			handlerGCError != nil || handlePruneError != nil || handleStatusError != nil {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).To(BeNil())
		}
	},
		Entry("handleMaster failed", fmt.Errorf("master error"), nil, nil, nil, nil, nil),
		Entry("handleWorker failed", nil, fmt.Errorf("worker error"), nil, nil, nil, nil),
		Entry("handleTopology failed", nil, nil, fmt.Errorf("topology error"), nil, nil, nil),
		Entry("handleGC failed", nil, nil, nil, fmt.Errorf("gc error"), nil, nil),
		Entry("handlePrune failed", nil, nil, nil, nil, fmt.Errorf("prune error"), nil),
		Entry("handleStatus failed", nil, nil, nil, nil, nil, fmt.Errorf("status error")),
		Entry("all components succeeded", nil, nil, nil, nil, nil, nil),
	)
})

var _ = Describe("handleMaster", func() {
	var (
		ctrl           *gomock.Controller
		clnt           *client.MockClient
		mockDeployment *deployment.MockDeploymentAPI
		nfdh           nodeFeatureDiscoveryHelperAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		mockDeployment = deployment.NewMockDeploymentAPI(ctrl)

		nfdh = newNodeFeatureDiscoveryHelperAPI(clnt, mockDeployment, nil, nil, scheme)
	})

	ctx := context.Background()

	It("should create new nfd-master deployment if it does not exist", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{}
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever")),
			mockDeployment.EXPECT().SetMasterDeploymentAsDesired(&nfdCR, gomock.Any()).Return(nil),
			clnt.EXPECT().Create(ctx, gomock.Any()).Return(nil),
		)

		err := nfdh.handleMaster(ctx, &nfdCR)
		Expect(err).To(BeNil())
	})

	It("deployment exists, no need to create it, update is not executed", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nfd-cr",
				Namespace: "test-namespace",
			},
		}
		existingDeployment := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: nfdCR.Namespace, Name: "nfd-master"},
		}
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, dp *appsv1.Deployment, _ ...ctrlclient.GetOption) error {
					dp.SetName(existingDeployment.Name)
					dp.SetNamespace(existingDeployment.Namespace)
					return nil
				},
			),
			mockDeployment.EXPECT().SetMasterDeploymentAsDesired(&nfdCR, &existingDeployment).Return(nil),
		)

		err := nfdh.handleMaster(ctx, &nfdCR)
		Expect(err).To(BeNil())
	})

	It("error flow, failed to populate deployment object", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{}
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever")),
			mockDeployment.EXPECT().SetMasterDeploymentAsDesired(&nfdCR, gomock.Any()).Return(fmt.Errorf("some error")),
		)

		err := nfdh.handleMaster(ctx, &nfdCR)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("handleWorker", func() {
	var (
		ctrl   *gomock.Controller
		clnt   *client.MockClient
		mockDS *daemonset.MockDaemonsetAPI
		mockCM *configmap.MockConfigMapAPI
		nfdh   nodeFeatureDiscoveryHelperAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		mockDS = daemonset.NewMockDaemonsetAPI(ctrl)
		mockCM = configmap.NewMockConfigMapAPI(ctrl)

		nfdh = newNodeFeatureDiscoveryHelperAPI(clnt, nil, mockDS, mockCM, scheme)
	})

	ctx := context.Background()
	nfdCR := nfdv1.NodeFeatureDiscovery{}

	It("both configmap and daemonset are missing, they should both be created", func() {
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever")),
			mockCM.EXPECT().SetWorkerConfigMapAsDesired(ctx, &nfdCR, gomock.Any()).Return(nil),
			clnt.EXPECT().Create(ctx, gomock.Any()).Return(nil),
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever")),
			mockDS.EXPECT().SetWorkerDaemonsetAsDesired(ctx, &nfdCR, gomock.Any()).Return(nil),
			clnt.EXPECT().Create(ctx, gomock.Any()).Return(nil),
		)

		err := nfdh.handleWorker(ctx, &nfdCR)
		Expect(err).To(BeNil())
	})

	It("worker config and daemonset exist, no need to create them, update is not executed", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nfd-cr",
				Namespace: "test-namespace",
			},
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				TopologyUpdater: true,
			},
		}
		existingDS := appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Namespace: nfdCR.Namespace, Name: "nfd-worker"},
		}
		existingCM := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: nfdCR.Namespace, Name: "nfd-worker"},
		}
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, cm *corev1.ConfigMap, _ ...ctrlclient.GetOption) error {
					cm.SetName(existingCM.Name)
					cm.SetNamespace(existingCM.Namespace)
					return nil
				},
			),
			mockCM.EXPECT().SetWorkerConfigMapAsDesired(ctx, &nfdCR, &existingCM).Return(nil),
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, ds *appsv1.DaemonSet, _ ...ctrlclient.GetOption) error {
					ds.SetName(existingDS.Name)
					ds.SetNamespace(existingDS.Namespace)
					return nil
				},
			),
			mockDS.EXPECT().SetWorkerDaemonsetAsDesired(ctx, &nfdCR, &existingDS).Return(nil),
		)

		err := nfdh.handleWorker(ctx, &nfdCR)
		Expect(err).To(BeNil())
	})

	It("error flow, failed to populate configmap object", func() {
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever")),
			mockCM.EXPECT().SetWorkerConfigMapAsDesired(ctx, &nfdCR, gomock.Any()).Return(fmt.Errorf("some error")),
		)

		err := nfdh.handleWorker(ctx, &nfdCR)
		Expect(err).To(HaveOccurred())
	})

	It("error flow, failed to populate daemonset object", func() {
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever")),
			mockCM.EXPECT().SetWorkerConfigMapAsDesired(ctx, &nfdCR, gomock.Any()).Return(nil),
			clnt.EXPECT().Create(ctx, gomock.Any()).Return(nil),
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever")),
			mockDS.EXPECT().SetWorkerDaemonsetAsDesired(ctx, &nfdCR, gomock.Any()).Return(fmt.Errorf("some error")),
		)

		err := nfdh.handleWorker(ctx, &nfdCR)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("handleTopology", func() {
	var (
		ctrl   *gomock.Controller
		clnt   *client.MockClient
		mockDS *daemonset.MockDaemonsetAPI
		nfdh   nodeFeatureDiscoveryHelperAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		mockDS = daemonset.NewMockDaemonsetAPI(ctrl)

		nfdh = newNodeFeatureDiscoveryHelperAPI(clnt, nil, mockDS, nil, scheme)
	})

	ctx := context.Background()

	It("should create new nfd-topology daemonset if it does not exist", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				TopologyUpdater: true,
			},
		}
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever")),
			mockDS.EXPECT().SetTopologyDaemonsetAsDesired(ctx, &nfdCR, gomock.Any()).Return(nil),
			clnt.EXPECT().Create(ctx, gomock.Any()).Return(nil),
		)

		err := nfdh.handleTopology(ctx, &nfdCR)
		Expect(err).To(BeNil())
	})

	It("topology daemonset exists, no need to create it, update is not executed", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nfd-cr",
				Namespace: "test-namespace",
			},
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				TopologyUpdater: true,
			},
		}
		existingDS := appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Namespace: nfdCR.Namespace, Name: "nfd-topology-updater"},
		}
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, ds *appsv1.DaemonSet, _ ...ctrlclient.GetOption) error {
					ds.SetName(existingDS.Name)
					ds.SetNamespace(existingDS.Namespace)
					return nil
				},
			),
			mockDS.EXPECT().SetTopologyDaemonsetAsDesired(ctx, &nfdCR, &existingDS).Return(nil),
		)

		err := nfdh.handleTopology(ctx, &nfdCR)
		Expect(err).To(BeNil())
	})

	It("error flow, failed to populate daemonset object", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				TopologyUpdater: true,
			},
		}
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever")),
			mockDS.EXPECT().SetTopologyDaemonsetAsDesired(ctx, &nfdCR, gomock.Any()).Return(fmt.Errorf("some error")),
		)

		err := nfdh.handleTopology(ctx, &nfdCR)
		Expect(err).To(HaveOccurred())
	})

	It("if TopologyUpdate not set - nothing to do", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{}

		err := nfdh.handleTopology(ctx, &nfdCR)
		Expect(err).To(BeNil())
	})
})

var _ = Describe("handleGC", func() {
	var (
		ctrl           *gomock.Controller
		clnt           *client.MockClient
		mockDeployment *deployment.MockDeploymentAPI
		nfdh           nodeFeatureDiscoveryHelperAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		mockDeployment = deployment.NewMockDeploymentAPI(ctrl)

		nfdh = newNodeFeatureDiscoveryHelperAPI(clnt, mockDeployment, nil, nil, scheme)
	})

	ctx := context.Background()

	It("should create new nfd-gc deployment if it does not exist", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{}
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever")),
			mockDeployment.EXPECT().SetGCDeploymentAsDesired(&nfdCR, gomock.Any()).Return(nil),
			clnt.EXPECT().Create(ctx, gomock.Any()).Return(nil),
		)

		err := nfdh.handleGC(ctx, &nfdCR)
		Expect(err).To(BeNil())
	})

	It("nfd-gc deployment exists, no need to create it, update is not executed", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nfd-cr",
				Namespace: "test-namespace",
			},
		}
		existingDeployment := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: nfdCR.Namespace, Name: "nfd-gc"},
		}
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, dp *appsv1.Deployment, _ ...ctrlclient.GetOption) error {
					dp.SetName(existingDeployment.Name)
					dp.SetNamespace(existingDeployment.Namespace)
					return nil
				},
			),
			mockDeployment.EXPECT().SetGCDeploymentAsDesired(&nfdCR, &existingDeployment).Return(nil),
		)

		err := nfdh.handleGC(ctx, &nfdCR)
		Expect(err).To(BeNil())
	})

	It("error flow, failed to populate nfd-gc deployment object", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{}
		gomock.InOrder(
			clnt.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever")),
			mockDeployment.EXPECT().SetGCDeploymentAsDesired(&nfdCR, gomock.Any()).Return(fmt.Errorf("some error")),
		)

		err := nfdh.handleGC(ctx, &nfdCR)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("hasFinalizer", func() {
	It("checking return status whether finalizer set or not", func() {
		nfdh := newNodeFeatureDiscoveryHelperAPI(nil, nil, nil, nil, nil)

		By("finalizers was empty")
		nfdCR := nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "nfd-cr",
				Finalizers: nil,
			},
		}
		res := nfdh.hasFinalizer(&nfdCR)
		Expect(res).To(BeFalse())

		By("finalizers exists, but NFD finalizer missing")
		nfdCR = nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "nfd-cr",
				Finalizers: []string{"some finalizer"},
			},
		}
		res = nfdh.hasFinalizer(&nfdCR)
		Expect(res).To(BeFalse())

		By("finalizers exists, but NFD finalizer present")
		nfdCR = nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "nfd-cr",
				Finalizers: []string{"some finalizer", finalizerLabel},
			},
		}
		res = nfdh.hasFinalizer(&nfdCR)
		Expect(res).To(BeTrue())
	})
})

var _ = Describe("setFinalizer", func() {
	var (
		ctrl *gomock.Controller
		clnt *client.MockClient
		nfdh nodeFeatureDiscoveryHelperAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		nfdh = newNodeFeatureDiscoveryHelperAPI(clnt, nil, nil, nil, nil)
	})

	It("checking the return status of setFinalizer function", func() {
		ctx := context.Background()

		By("Updating the NFD instance fails, original finalizers was empty")
		nfdCR := nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "nfd-cr",
				Finalizers: nil,
			},
		}
		expectedCR := nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "nfd-cr",
				Finalizers: []string{finalizerLabel},
			},
		}
		clnt.EXPECT().Update(ctx, &expectedCR).Return(fmt.Errorf("some error"))
		err := nfdh.setFinalizer(ctx, &nfdCR)
		Expect(err).ToNot(BeNil())

		By("Updating the NFD instance succeeds")
		nfdCR = nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "nfd-cr",
				Finalizers: []string{"some finalizer"},
			},
		}
		expectedCR = nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "nfd-cr",
				Finalizers: []string{"some finalizer", finalizerLabel},
			},
		}
		clnt.EXPECT().Update(ctx, &expectedCR).Return(nil)
		err = nfdh.setFinalizer(ctx, &nfdCR)
		Expect(err).To(BeNil())
	})
})

var _ = Describe("finalizeComponents", func() {
	var (
		ctrl           *gomock.Controller
		clnt           *client.MockClient
		mockDeployment *deployment.MockDeploymentAPI
		mockDS         *daemonset.MockDaemonsetAPI
		mockCM         *configmap.MockConfigMapAPI
		nfdh           nodeFeatureDiscoveryHelperAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		mockDeployment = deployment.NewMockDeploymentAPI(ctrl)
		mockDS = daemonset.NewMockDaemonsetAPI(ctrl)
		mockCM = configmap.NewMockConfigMapAPI(ctrl)

		nfdh = newNodeFeatureDiscoveryHelperAPI(clnt, mockDeployment, mockDS, mockCM, scheme)
	})

	ctx := context.Background()
	namespace := "test-namespace"
	nfdCR := nfdv1.NodeFeatureDiscovery{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
		Spec: nfdv1.NodeFeatureDiscoverySpec{
			TopologyUpdater: true,
		},
	}

	DescribeTable("check finalization normal and error flows", func(deleteWorkerDSError,
		deleteWorkerCMError,
		deleteTopologyDSError,
		deleteMasterDeploymentError,
		deleteGCDeploymentError,
		updateError bool) {

		controllerutil.AddFinalizer(&nfdCR, finalizerLabel)

		if deleteWorkerDSError {
			mockDS.EXPECT().DeleteDaemonSet(ctx, namespace, "nfd-worker").Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		mockDS.EXPECT().DeleteDaemonSet(ctx, namespace, "nfd-worker").Return(nil)
		if deleteWorkerCMError {
			mockCM.EXPECT().DeleteConfigMap(ctx, namespace, "nfd-worker").Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		mockCM.EXPECT().DeleteConfigMap(ctx, namespace, "nfd-worker").Return(nil)
		if deleteTopologyDSError {
			mockDS.EXPECT().DeleteDaemonSet(ctx, namespace, "nfd-topology-updater").Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		mockDS.EXPECT().DeleteDaemonSet(ctx, namespace, "nfd-topology-updater").Return(nil)
		if deleteMasterDeploymentError {
			mockDeployment.EXPECT().DeleteDeployment(ctx, namespace, "nfd-master").Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		mockDeployment.EXPECT().DeleteDeployment(ctx, namespace, "nfd-master").Return(nil)
		if deleteGCDeploymentError {
			mockDeployment.EXPECT().DeleteDeployment(ctx, namespace, "nfd-gc").Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		mockDeployment.EXPECT().DeleteDeployment(ctx, namespace, "nfd-gc").Return(nil)
		if updateError {
			clnt.EXPECT().Update(ctx, gomock.Any()).Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		clnt.EXPECT().Update(ctx, gomock.Any()).Return(nil)

	executeTestFunction:

		err := nfdh.finalizeComponents(ctx, &nfdCR)

		if deleteGCDeploymentError || deleteWorkerDSError || deleteWorkerCMError ||
			deleteTopologyDSError || deleteMasterDeploymentError || updateError {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).To(BeNil())
		}
	},
		Entry("delete worker daemonset failed", true, false, false, false, false, false),
		Entry("delete worker configmap failed", false, true, false, false, false, false),
		Entry("delete topology daemonset failed", false, false, true, false, false, false),
		Entry("delete master deployment failed", false, false, false, true, false, false),
		Entry("delete gc deployment failed", false, false, false, false, true, false),
		Entry("updating removed finalizer failed", false, false, false, false, false, true),
		Entry("finalization flow was succesful", false, false, false, false, false, false),
	)
})
