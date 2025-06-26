// DELETE THIS FILE. It is a duplicate and causes build errors.
/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package instanceset

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// InstanceSetReconciler reconciles an InstanceSet object
type InstanceSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete

// Reconcile handles InstanceSet reconciliation
func (r *InstanceSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling InstanceSet", "name", req.NamespacedName)

	// Fetch the InstanceSet resource
	instanceSet := &workloadsv1alpha1.InstanceSet{}
	if err := r.Get(ctx, req.NamespacedName, instanceSet); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, likely deleted
			return ctrl.Result{}, nil
		}
		// Error reading the object
		logger.Error(err, "Failed to get InstanceSet")
		return ctrl.Result{}, err
	}

	// Recover stuck pods if any
	if err := r.recoverStuckPods(ctx, instanceSet); err != nil {
		logger.Error(err, "failed to recover stuck pods")
		r.Recorder.Event(instanceSet, corev1.EventTypeWarning, "RecoveryError", err.Error())
		return ctrl.Result{}, err
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(instanceSet, constant.DBResourceFinalizerName) {
		controllerutil.AddFinalizer(instanceSet, constant.DBResourceFinalizerName)
		if err := r.Update(ctx, instanceSet); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Handle deletion
	if !instanceSet.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, instanceSet)
	}

	// Get all pods owned by this InstanceSet
	podList := &corev1.PodList{}
	if err := r.listPods(ctx, instanceSet, podList); err != nil {
		logger.Error(err, "Failed to list pods")
		return ctrl.Result{}, err
	}

	// Handle recovery operations if needed
	if err := r.handleRecovery(ctx, instanceSet, podList); err != nil {
		logger.Error(err, "Failed to handle recovery")
		return ctrl.Result{}, err
	}

	// Update status
	if err := r.updateStatus(ctx, instanceSet, podList); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	logger.Info("InstanceSet reconciliation completed")
	return ctrl.Result{}, nil
}

// handleDeletion processes the deletion of an InstanceSet
func (r *InstanceSetReconciler) handleDeletion(ctx context.Context, instanceSet *workloadsv1alpha1.InstanceSet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Perform cleanup actions here

	// Remove finalizer
	controllerutil.RemoveFinalizer(instanceSet, constant.DBResourceFinalizerName)
	if err := r.Update(ctx, instanceSet); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// listPods lists pods owned by the InstanceSet
func (r *InstanceSetReconciler) listPods(ctx context.Context, instanceSet *workloadsv1alpha1.InstanceSet, podList *corev1.PodList) error {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			workloadsv1alpha1.InstanceSetNameLabel: instanceSet.Name,
		},
	}

	selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return err
	}

	return r.List(ctx, podList, &client.ListOptions{
		Namespace:     instanceSet.Namespace,
		LabelSelector: selector,
	})
}

// handleRecovery manages recovery operations for stuck or outdated pods
func (r *InstanceSetReconciler) handleRecovery(ctx context.Context, instanceSet *workloadsv1alpha1.InstanceSet, podList *corev1.PodList) error {
	recoveryManager := NewRecoveryManager(ctx, r.Client, podList)

	// Find outdated pending pods
	outdatedPods := recoveryManager.FindOutdatedPendingPods(instanceSet.Status.CurrentRevision)
	if len(outdatedPods) > 0 {
		logger := log.FromContext(ctx)
		logger.Info("Found outdated pending pods", "count", len(outdatedPods))
		errors := recoveryManager.DeletePods(outdatedPods)
		if len(errors) > 0 {
			// Just log errors but continue processing
			for _, err := range errors {
				logger.Error(err, "Error during pod deletion")
			}
		}
	}

	// Find stuck pending pods
	stuckPods := recoveryManager.FindStuckPendingPods()
	if len(stuckPods) > 0 {
		logger := log.FromContext(ctx)
		logger.Info("Found stuck pending pods", "count", len(stuckPods))
		errors := recoveryManager.DeletePods(stuckPods)
		if len(errors) > 0 {
			// Just log errors but continue processing
			for _, err := range errors {
				logger.Error(err, "Error during pod deletion")
			}
		}
	}

	return nil
}

// updateStatus updates the status of the InstanceSet
func (r *InstanceSetReconciler) updateStatus(ctx context.Context, instanceSet *workloadsv1alpha1.InstanceSet, podList *corev1.PodList) error {
	// Calculate ready replicas
	readyReplicas := 0
	for i := range podList.Items {
		if isPodReady(&podList.Items[i]) {
			readyReplicas++
		}
	}

	// Update status if needed
	if instanceSet.Status.ReadyReplicas != readyReplicas {
		instanceSet.Status.ReadyReplicas = readyReplicas
		return r.Status().Update(ctx, instanceSet)
	}

	return nil
}

// isPodReady returns true if a pod is ready
func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

// SetupWithManager sets up the controller with the Manager
func (r *InstanceSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workloadsv1alpha1.InstanceSet{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
