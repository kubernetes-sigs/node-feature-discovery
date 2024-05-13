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
	"errors"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
	"sigs.k8s.io/node-feature-discovery-operator/internal/configmap"
	"sigs.k8s.io/node-feature-discovery-operator/internal/daemonset"
	"sigs.k8s.io/node-feature-discovery-operator/internal/deployment"
	"sigs.k8s.io/node-feature-discovery-operator/internal/job"
)

const finalizerLabel = "nfd-finalizer"

// NodeFeatureDiscoveryReconciler reconciles a NodeFeatureDiscovery object
type nodeFeatureDiscoveryReconciler struct {
	helper nodeFeatureDiscoveryHelperAPI
}

func NewNodeFeatureDiscoveryReconciler(client client.Client, deploymentAPI deployment.DeploymentAPI, daemonsetAPI daemonset.DaemonsetAPI,
	configmapAPI configmap.ConfigMapAPI, jobAPI job.JobAPI, scheme *runtime.Scheme) *nodeFeatureDiscoveryReconciler {
	helper := newNodeFeatureDiscoveryHelperAPI(client, deploymentAPI, daemonsetAPI, configmapAPI, jobAPI, scheme)
	return &nodeFeatureDiscoveryReconciler{
		helper: helper,
	}
}

// SetupWithManager sets up the controller with a specified manager responsible for
// initializing shared dependencies (like caches and clients)
func (r *nodeFeatureDiscoveryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p := getPredicates()

	// watch for all events on NodeFeatureDiscovery and for
	// update and delete events for the resource created by operator
	return ctrl.NewControllerManagedBy(mgr).
		For(&nfdv1.NodeFeatureDiscovery{}).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(p)).
		Owns(&appsv1.DaemonSet{}, builder.WithPredicates(p)).
		Owns(&corev1.ConfigMap{}, builder.WithPredicates(p)).
		Owns(&batchv1.Job{}, builder.WithPredicates(p)).
		Complete(reconcile.AsReconciler[*nfdv1.NodeFeatureDiscovery](mgr.GetClient(), r))
}

func getPredicates() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isControlledByNFD(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isControlledByNFD(e.Object)
		},
	}
}

func isControlledByNFD(obj client.Object) bool {
	controller := metav1.GetControllerOf(obj)
	if controller == nil {
		return false
	}
	nfdKind := reflect.TypeOf(nfdv1.NodeFeatureDiscovery{}).Name()
	return controller.Kind == nfdKind
}

// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nfd.k8s-sigs.io,resources=nodefeaturerules,verbs=get;list;watch
// +kubebuilder:rbac:groups=nfd.kubernetes.io,resources=nodefeaturediscoveries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nfd.kubernetes.io,resources=nodefeaturediscoveries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=nfd.kubernetes.io,resources=nodefeaturediscoveries/finalizers,verbs=update

// Reconcile moves the current state of the cluster closer to the desired state.
// It creates/pataches the NFD components ( master, worker, topology, prune, GC) in accordance with
// NFD CR Spec. In addition it also updates the Status of the NFD CR
func (r *nodeFeatureDiscoveryReconciler) Reconcile(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) (ctrl.Result, error) {
	res := ctrl.Result{}
	logger := ctrl.LoggerFrom(ctx).WithValues("instance namespace", nfdInstance.Namespace, "instance name", nfdInstance.Name)

	if nfdInstance.DeletionTimestamp != nil {
		// NFD CR is being deleted
		err := r.helper.finalizeComponents(ctx, nfdInstance)
		if err != nil {
			return res, fmt.Errorf("failed to finalize components for %s/%s: %w", nfdInstance.Namespace, nfdInstance.Name, err)
		}
		done, err := r.helper.handlePrune(ctx, nfdInstance)
		if err != nil {
			return res, fmt.Errorf("failed to handle pruning for %s/%s: %w", nfdInstance.Namespace, nfdInstance.Name, err)
		}
		if !done {
			// reconcile will be called again when prune job has been completed
			return res, nil
		}
		return res, r.helper.removeFinalizer(ctx, nfdInstance)
	}

	// If the finalizer doesn't exist, add it.
	if !r.helper.hasFinalizer(nfdInstance) {
		return res, r.helper.setFinalizer(ctx, nfdInstance)
	}

	errs := make([]error, 0, 10)
	logger.Info("reconciling master component")
	err := r.helper.handleMaster(ctx, nfdInstance)
	errs = append(errs, err)

	logger.Info("reconciling worker component")
	err = r.helper.handleWorker(ctx, nfdInstance)
	errs = append(errs, err)

	logger.Info("reconciling topology components")
	err = r.helper.handleTopology(ctx, nfdInstance)
	errs = append(errs, err)

	logger.Info("reconciling garbage collector")
	err = r.helper.handleGC(ctx, nfdInstance)
	errs = append(errs, err)

	logger.Info("reconciling NFD status")
	err = r.helper.handleStatus(ctx, nfdInstance)
	errs = append(errs, err)

	return res, errors.Join(errs...)
}

//go:generate mockgen -source=nodefeaturediscovery_reconciler.go -package=new_controllers -destination=mock_nodefeaturediscovery_reconciler.go nodeFeatureDiscoveryHelperAPI

type nodeFeatureDiscoveryHelperAPI interface {
	finalizeComponents(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error
	hasFinalizer(nfdInstance *nfdv1.NodeFeatureDiscovery) bool
	setFinalizer(ctx context.Context, instance *nfdv1.NodeFeatureDiscovery) error
	removeFinalizer(ctx context.Context, instance *nfdv1.NodeFeatureDiscovery) error
	handleMaster(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error
	handleWorker(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error
	handleTopology(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error
	handleGC(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error
	handlePrune(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) (bool, error)
	handleStatus(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error
}

type nodeFeatureDiscoveryHelper struct {
	client        client.Client
	deploymentAPI deployment.DeploymentAPI
	daemonsetAPI  daemonset.DaemonsetAPI
	configmapAPI  configmap.ConfigMapAPI
	jobAPI        job.JobAPI
	scheme        *runtime.Scheme
}

func newNodeFeatureDiscoveryHelperAPI(client client.Client, deploymentAPI deployment.DeploymentAPI, daemonsetAPI daemonset.DaemonsetAPI,
	configmapAPI configmap.ConfigMapAPI, jobAPI job.JobAPI, scheme *runtime.Scheme) nodeFeatureDiscoveryHelperAPI {
	return &nodeFeatureDiscoveryHelper{
		client:        client,
		deploymentAPI: deploymentAPI,
		daemonsetAPI:  daemonsetAPI,
		configmapAPI:  configmapAPI,
		jobAPI:        jobAPI,
		scheme:        scheme,
	}
}

func (nfdh *nodeFeatureDiscoveryHelper) finalizeComponents(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error {
	err := nfdh.daemonsetAPI.DeleteDaemonSet(ctx, nfdInstance.Namespace, "nfd-worker")
	if err != nil {
		return fmt.Errorf("failed to delete worker daemonset: %w", err)
	}

	err = nfdh.configmapAPI.DeleteConfigMap(ctx, nfdInstance.Namespace, "nfd-worker")
	if err != nil {
		return fmt.Errorf("failed to delete worker config map: %w", err)
	}

	if nfdInstance.Spec.TopologyUpdater {
		err = nfdh.daemonsetAPI.DeleteDaemonSet(ctx, nfdInstance.Namespace, "nfd-topology-updater")
		if err != nil {
			return fmt.Errorf("failed to delete topology-updater daemonset: %w", err)
		}
	}
	err = nfdh.deploymentAPI.DeleteDeployment(ctx, nfdInstance.Namespace, "nfd-master")
	if err != nil {
		return fmt.Errorf("failed to delete master deployment: %w", err)
	}

	return nfdh.deploymentAPI.DeleteDeployment(ctx, nfdInstance.Namespace, "nfd-gc")
}

func (nfdh *nodeFeatureDiscoveryHelper) hasFinalizer(nfdInstance *nfdv1.NodeFeatureDiscovery) bool {
	return controllerutil.ContainsFinalizer(nfdInstance, finalizerLabel)
}

func (nfdh *nodeFeatureDiscoveryHelper) setFinalizer(ctx context.Context, instance *nfdv1.NodeFeatureDiscovery) error {
	instance.Finalizers = append(instance.Finalizers, finalizerLabel)
	return nfdh.client.Update(ctx, instance)
}

func (nfdh *nodeFeatureDiscoveryHelper) removeFinalizer(ctx context.Context, instance *nfdv1.NodeFeatureDiscovery) error {
	updated := controllerutil.RemoveFinalizer(instance, finalizerLabel)
	if updated {
		return nfdh.client.Update(ctx, instance)
	}
	return nil
}

func (nfdh *nodeFeatureDiscoveryHelper) handleMaster(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error {
	masterDep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "nfd-master", Namespace: nfdInstance.Namespace},
	}
	opRes, err := controllerutil.CreateOrPatch(ctx, nfdh.client, &masterDep, func() error {
		return nfdh.deploymentAPI.SetMasterDeploymentAsDesired(nfdInstance, &masterDep)
	})

	if err != nil {
		return fmt.Errorf("failed to reconcile master deployment %s/%s: %w", nfdInstance.Namespace, nfdInstance.Name, err)
	}
	ctrl.LoggerFrom(ctx).Info("reconciled master deployment", "namespace", nfdInstance.Namespace, "name", nfdInstance.Name, "result", opRes)
	return nil
}

func (nfdh *nodeFeatureDiscoveryHelper) handleWorker(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error {
	logger := ctrl.LoggerFrom(ctx)

	workerCM := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "nfd-worker", Namespace: nfdInstance.Namespace},
	}
	cmRes, err := controllerutil.CreateOrPatch(ctx, nfdh.client, &workerCM, func() error {
		return nfdh.configmapAPI.SetWorkerConfigMapAsDesired(ctx, nfdInstance, &workerCM)
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile worker configmap %s/%s: %w", nfdInstance.Namespace, nfdInstance.Name, err)
	}
	logger.Info("reconciled worker ConfigMap", "namespace", nfdInstance.Namespace, "name", nfdInstance.Name, "result", cmRes)

	workerDS := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "nfd-worker", Namespace: nfdInstance.Namespace},
	}
	opRes, err := controllerutil.CreateOrPatch(ctx, nfdh.client, &workerDS, func() error {
		return nfdh.daemonsetAPI.SetWorkerDaemonsetAsDesired(ctx, nfdInstance, &workerDS)
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile worker DaemonSet %s/%s: %w", nfdInstance.Namespace, nfdInstance.Name, err)
	}

	logger.Info("reconciled worker DaemonSet", "namespace", nfdInstance.Namespace, "name", nfdInstance.Name, "result", opRes)

	return nil
}

func (nfdh *nodeFeatureDiscoveryHelper) handleTopology(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error {
	if !nfdInstance.Spec.TopologyUpdater {
		return nil
	}
	topologyDS := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "nfd-topology-updater", Namespace: nfdInstance.Namespace},
	}
	opRes, err := controllerutil.CreateOrPatch(ctx, nfdh.client, &topologyDS, func() error {
		return nfdh.daemonsetAPI.SetTopologyDaemonsetAsDesired(ctx, nfdInstance, &topologyDS)
	})

	if err != nil {
		return fmt.Errorf("failed to reconcile topology daemonset %s/%s: %w", nfdInstance.Namespace, nfdInstance.Name, err)
	}
	ctrl.LoggerFrom(ctx).Info("reconciled topoplogy daemonset", "namespace", nfdInstance.Namespace, "name", nfdInstance.Name, "result", opRes)
	return nil
}

func (nfdh *nodeFeatureDiscoveryHelper) handleGC(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error {
	gcDep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "nfd-gc", Namespace: nfdInstance.Namespace},
	}
	opRes, err := controllerutil.CreateOrPatch(ctx, nfdh.client, &gcDep, func() error {
		return nfdh.deploymentAPI.SetGCDeploymentAsDesired(nfdInstance, &gcDep)
	})

	if err != nil {
		return fmt.Errorf("failed to reconcile nfd-gc deployment %s/%s: %w", nfdInstance.Namespace, nfdInstance.Name, err)
	}
	ctrl.LoggerFrom(ctx).Info("reconciled nfd-gc deployment", "namespace", nfdInstance.Namespace, "name", nfdInstance.Name, "result", opRes)
	return nil
}

func (nfdh *nodeFeatureDiscoveryHelper) handlePrune(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) (bool, error) {
	if !nfdInstance.Spec.PruneOnDelete {
		return true, nil
	}

	pruneJob, err := nfdh.jobAPI.GetJob(ctx, nfdInstance.Namespace, "nfd-prune")
	if err != nil {
		if k8serrors.IsNotFound(err) {
			err = nfdh.jobAPI.CreatePruneJob(ctx, nfdInstance)
			if err != nil {
				return false, fmt.Errorf("failed to create nfd-prune job: %w", err)
			}
			return false, nil
		}
		return false, fmt.Errorf("failed to get nfd-prune job: %w", err)
	}

	var returnErr error
	done := false
	if pruneJob.Status.Succeeded > 0 {
		done = true
	}
	if pruneJob.Status.Failed > 0 {
		returnErr = fmt.Errorf("prune job's pod has failed")
	}

	// no need to explicitly delete Prune job,
	// it will be deleted by K8S scheduler once NFD CR is deleted from etcd
	return done, returnErr
}

func (nfdh *nodeFeatureDiscoveryHelper) handleStatus(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery) error {
	return nil
}
