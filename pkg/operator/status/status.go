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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
	"sigs.k8s.io/node-feature-discovery-operator/internal/daemonset"
	"sigs.k8s.io/node-feature-discovery-operator/internal/deployment"
)

const (
	conditionStatusProgressing = "progressing"
	conditionStatusDegraded    = "degrading"
	conditionStatusAvailable   = "available"

	conditionFailedGettingNFDWorkerDaemonSet = "FailedGettingNFDWorkerDaemonSet"
	conditionNFDWorkerDaemonSetDegraded      = "NFDWorkerDaemonSetDegraded"
	conditionNFDWorkerDaemonSetProgressing   = "NFDWorkerDaemonSetProgressing"

	conditionFailedGettingNFDTopologyDaemonSet = "FailedGettingNFDTopologyDaemonSet"
	conditionNFDTopologyDaemonSetDegraded      = "NFDTopologyDaemonSetDegraded"
	conditionNFDTopologyDaemonSetProgressing   = "NFDTopologyDaemonSetProgressing"

	conditionFailedGettingNFDMasterDeployment = "FailedGettingNFDMasterDeployment"
	conditionNFDMasterDeploymentDegraded      = "NFDMasterDeploymentDegraded"
	conditionNFDMasterDeploymentProgressing   = "NFDMasterDeploymentProgressing"

	conditionFailedGettingNFDGCDeployment = "FailedGettingNFDGCDeployment"
	conditionNFDGCDeploymentDegraded      = "NFDGCDegraded"
	conditionNFDGCDeploymentProgressing   = "NFDGCDeploymentProgressing"

	conditionIsFalseReason = "ConditionNotBeingMetCurrently"

	// ConditionAvailable indicates that the resources maintained by the operator,
	// is functional and available in the cluster.
	conditionAvailable string = "Available"

	// ConditionProgressing indicates that the operator is actively making changes to the resources maintained by the
	// operator
	conditionProgressing string = "Progressing"

	// ConditionDegraded indicates that the resources maintained by the operator are not functioning completely.
	// An example of a degraded state would be if not all pods in a deployment were running.
	// It may still be available, but it is degraded
	conditionDegraded string = "Degraded"

	// ConditionUpgradeable indicates whether the resources maintained by the operator are in a state that is safe to upgrade.
	// When `False`, the resources maintained by the operator should not be upgraded and the
	// message field should contain a human readable description of what the administrator should do to
	// allow the operator to successfully update the resources maintained by the operator.
	conditionUpgradeable string = "Upgradeable"
)

//go:generate mockgen -source=status.go -package=status -destination=mock_status.go StatusAPI

type StatusAPI interface {
	GetConditions(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) []metav1.Condition
	AreConditionsEqual(prevConditions, newConditions []metav1.Condition) bool
}

type status struct {
	helper statusHelperAPI
}

func NewStatusAPI(deploymentAPI deployment.DeploymentAPI, daemonsetAPI daemonset.DaemonsetAPI) StatusAPI {
	helper := newStatusHelperAPI(deploymentAPI, daemonsetAPI)
	return &status{
		helper: helper,
	}
}

func (s *status) GetConditions(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) []metav1.Condition {
	// get worker daemonset conditions
	nonAvailableConditions := s.helper.getWorkerNotAvailableConditions(ctx, nfdInstance)
	if nonAvailableConditions != nil {
		return nonAvailableConditions
	}
	// get master deployment conditions
	nonAvailableConditions = s.helper.getMasterNotAvailableConditions(ctx, nfdInstance)
	if nonAvailableConditions != nil {
		return nonAvailableConditions
	}
	// get GC deployment conditions
	nonAvailableConditions = s.helper.getGCNotAvailableConditions(ctx, nfdInstance)
	if nonAvailableConditions != nil {
		return nonAvailableConditions
	}
	// get topology, if needed
	if nfdInstance.Spec.TopologyUpdater {
		nonAvailableConditions := s.helper.getTopologyNotAvailableConditions(ctx, nfdInstance)
		if nonAvailableConditions != nil {
			return nonAvailableConditions
		}
	}

	return getAvailableConditions()
}

func (s *status) AreConditionsEqual(prevConditions, newConditions []metav1.Condition) bool {
	for _, newCondition := range newConditions {
		oldCondition := meta.FindStatusCondition(prevConditions, newCondition.Type)
		if oldCondition == nil {
			return false
		}
		// Ignore timestamps
		if oldCondition.Status != newCondition.Status ||
			oldCondition.Reason != newCondition.Reason ||
			oldCondition.Message != newCondition.Message {
			return false
		}
	}
	return true
}

//go:generate mockgen -source=status.go -package=status -destination=mock_status.go statusHelperAPI

type statusHelperAPI interface {
	getWorkerNotAvailableConditions(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) []metav1.Condition
	getTopologyNotAvailableConditions(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) []metav1.Condition
	getMasterNotAvailableConditions(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) []metav1.Condition
	getGCNotAvailableConditions(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) []metav1.Condition
}

type statusHelper struct {
	deploymentAPI deployment.DeploymentAPI
	daemonsetAPI  daemonset.DaemonsetAPI
}

func newStatusHelperAPI(deploymentAPI deployment.DeploymentAPI, daemonsetAPI daemonset.DaemonsetAPI) statusHelperAPI {
	return &statusHelper{
		deploymentAPI: deploymentAPI,
		daemonsetAPI:  daemonsetAPI,
	}
}

func (sh *statusHelper) getWorkerNotAvailableConditions(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) []metav1.Condition {
	return sh.getDaemonSetNotAvailableConditions(ctx,
		nfdInstance.Namespace,
		"nfd-worker",
		conditionFailedGettingNFDWorkerDaemonSet,
		conditionNFDWorkerDaemonSetDegraded,
		conditionNFDWorkerDaemonSetProgressing)
}

func (sh *statusHelper) getTopologyNotAvailableConditions(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) []metav1.Condition {
	return sh.getDaemonSetNotAvailableConditions(ctx,
		nfdInstance.Namespace,
		"nfd-topology-updater",
		conditionFailedGettingNFDTopologyDaemonSet,
		conditionNFDTopologyDaemonSetDegraded,
		conditionNFDTopologyDaemonSetProgressing)
}

func (sh *statusHelper) getDaemonSetNotAvailableConditions(ctx context.Context,
	dsNamespace,
	dsName,
	failedToGetDSReason,
	dsDegradedReason,
	dsProgressingReason string) []metav1.Condition {

	ds, err := sh.daemonsetAPI.GetDaemonSet(ctx, dsNamespace, dsName)
	if err != nil {
		return getDegradedConditions(failedToGetDSReason, err.Error())
	}
	conditionsStatus, message := getDaemonSetConditions(ds)
	if conditionsStatus == conditionStatusDegraded {
		return getDegradedConditions(dsDegradedReason, message)
	} else if conditionsStatus == conditionStatusProgressing {
		return getProgressingConditions(dsProgressingReason, message)
	}
	return nil
}

func (sh *statusHelper) getMasterNotAvailableConditions(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) []metav1.Condition {
	return sh.getDeploymentNotAvailableConditions(ctx,
		nfdInstance.Namespace,
		"nfd-master",
		conditionFailedGettingNFDMasterDeployment,
		conditionNFDMasterDeploymentDegraded,
		conditionNFDMasterDeploymentProgressing)
}

func (sh *statusHelper) getGCNotAvailableConditions(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) []metav1.Condition {
	return sh.getDeploymentNotAvailableConditions(ctx,
		nfdInstance.Namespace,
		"nfd-gc",
		conditionFailedGettingNFDGCDeployment,
		conditionNFDGCDeploymentDegraded,
		conditionNFDGCDeploymentProgressing)

}

func (sh *statusHelper) getDeploymentNotAvailableConditions(ctx context.Context,
	deploymentNamespace,
	deploymentName,
	failedToGetDeploymentReason,
	deploymentDegradedReason,
	deploymentProgressingReason string) []metav1.Condition {

	dep, err := sh.deploymentAPI.GetDeployment(ctx, deploymentNamespace, deploymentName)
	if err != nil {
		return getDegradedConditions(failedToGetDeploymentReason, err.Error())
	}
	conditionsStatus, message := getDeploymentConditions(dep)
	if conditionsStatus == conditionStatusDegraded {
		return getDegradedConditions(deploymentDegradedReason, message)
	} else if conditionsStatus == conditionStatusProgressing {
		return getProgressingConditions(deploymentProgressingReason, message)
	}
	return nil
}

func getDaemonSetConditions(ds *appsv1.DaemonSet) (string, string) {
	if ds.Status.DesiredNumberScheduled == 0 {
		return conditionStatusDegraded, "number of desired nodes for scheduling is 0"
	}
	if ds.Status.CurrentNumberScheduled == 0 {
		return conditionStatusDegraded, "0 nodes have pods scheduled"
	}
	if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
		return conditionStatusAvailable, ""
	}
	return conditionStatusProgressing, "ds is progressing"
}

func getDeploymentConditions(dep *appsv1.Deployment) (string, string) {
	if dep.Status.AvailableReplicas == 0 {
		return conditionStatusDegraded, "number of available pods is 0"
	}
	return conditionStatusAvailable, ""
}

// getAvailableConditions returns a list of Condition objects and marks
// every condition as FALSE except for ConditionAvailable so that the
// reconciler can determine that the resource is available.
func getAvailableConditions() []metav1.Condition {
	now := time.Now()
	return []metav1.Condition{
		{
			Type:               conditionAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "AllInstanceComponentsAreDeployedSuccessfuly",
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               conditionUpgradeable,
			Status:             metav1.ConditionTrue,
			Reason:             "CanBeUpgraded",
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               conditionProgressing,
			Status:             metav1.ConditionFalse,
			Reason:             conditionIsFalseReason,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               conditionDegraded,
			Status:             metav1.ConditionFalse,
			Reason:             conditionIsFalseReason,
			LastTransitionTime: metav1.Time{Time: now},
		},
	}
}

// getDegradedConditions returns a list of conditions.Condition objects and marks
// every condition as FALSE except for conditions.ConditionDegraded so that the
// reconciler can determine that the resource is degraded.
func getDegradedConditions(reason string, message string) []metav1.Condition {
	now := time.Now()
	return []metav1.Condition{
		{
			Type:               conditionAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             conditionIsFalseReason,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               conditionUpgradeable,
			Status:             metav1.ConditionFalse,
			Reason:             conditionIsFalseReason,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               conditionProgressing,
			Status:             metav1.ConditionFalse,
			Reason:             conditionIsFalseReason,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               conditionDegraded,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             reason,
			Message:            message,
		},
	}
}

// getProgressingConditions returns a list of Condition objects and marks
// every condition as FALSE except for ConditionProgressing so that the
// reconciler can determine that the resource is progressing.
func getProgressingConditions(reason string, message string) []metav1.Condition {
	now := time.Now()
	return []metav1.Condition{
		{
			Type:               conditionAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             conditionIsFalseReason,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               conditionUpgradeable,
			Status:             metav1.ConditionFalse,
			Reason:             conditionIsFalseReason,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               conditionProgressing,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             reason,
			Message:            message,
		},
		{
			Type:               conditionDegraded,
			Status:             metav1.ConditionFalse,
			Reason:             conditionIsFalseReason,
			LastTransitionTime: metav1.Time{Time: now},
		},
	}
}
