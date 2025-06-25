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

package apps

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// InstanceSetReconciler reconciles a InstanceSet object
type InstanceSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=instancesets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=instancesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=instancesets/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *InstanceSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the InstanceSet instance
	instanceSet := &appsv1alpha1.InstanceSet{}
	if err := r.Get(ctx, req.NamespacedName, instanceSet); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Your reconciliation logic here
	return r.reconcileInstanceSet(ctx, instanceSet)
}

func (r *InstanceSetReconciler) reconcileInstanceSet(ctx context.Context, instanceSet *appsv1alpha1.InstanceSet) (ctrl.Result, error) {
	// existing reconciliation logic
	// ...

	// Check for Pods that are stuck in Pending phase due to scheduling issues
	if err := r.handleStuckPods(ctx, instanceSet); err != nil {
		return ctrl.Result{}, err
	}

	var status appsv1alpha1.InstanceSetStatus
	// Populate status with the current state
	// ...

	// update InstanceSet status
	if err := r.updateStatus(ctx, instanceSet, &status); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// updateStatus updates the status of the InstanceSet
func (r *InstanceSetReconciler) updateStatus(ctx context.Context, instanceSet *appsv1alpha1.InstanceSet, status *appsv1alpha1.InstanceSetStatus) error {
	instanceSet.Status = *status
	return r.Status().Update(ctx, instanceSet)
}

// handleStuckPods checks for Pods that are stuck in Pending phase
// and recreates them if a new Template revision is available
func (r *InstanceSetReconciler) handleStuckPods(ctx context.Context, instanceSet *appsv1alpha1.InstanceSet) error {
	log := ctrl.LoggerFrom(ctx)

	// Get all Pods for this InstanceSet
	pods, err := r.getPodsForInstanceSet(ctx, instanceSet)
	if err != nil {
		return err
	}

	currentRevision := instanceSet.Status.CurrentRevision
	if currentRevision == "" {
		return nil // No revision yet, nothing to do
	}

	// Check for Pods stuck in Pending state
	stuckPendingPods := make([]corev1.Pod, 0)
	for _, pod := range pods {
		// Check if Pod is stuck in Pending phase for too long
		if pod.Status.Phase == corev1.PodPending {
			// Check if pod has a different template revision than current
			podRevision := pod.Labels[appsv1alpha1.InstanceSetRevisionLabel]
			if podRevision != currentRevision {
				// Pod is using old revision and is stuck, mark for recreation
				stuckPendingPods = append(stuckPendingPods, pod)
				continue
			}

			// Check if pod has been pending for too long (more than 5 minutes)
			pendingTooLong := false
			if pod.Status.StartTime != nil {
				pendingDuration := time.Since(pod.Status.StartTime.Time)
				pendingTooLong = pendingDuration > 5*time.Minute
			} else if !pod.CreationTimestamp.IsZero() {
				pendingDuration := time.Since(pod.CreationTimestamp.Time)
				pendingTooLong = pendingDuration > 5*time.Minute
			}

			if pendingTooLong {
				// Check for scheduling issues in pod conditions
				for _, condition := range pod.Status.Conditions {
					if condition.Type == corev1.PodScheduled &&
						condition.Status == corev1.ConditionFalse &&
						condition.Reason == "Unschedulable" {
						stuckPendingPods = append(stuckPendingPods, pod)
						break
					}
				}
			}
		}
	}

	// If we found stuck pods, delete them so they can be recreated with the new template
	for _, pod := range stuckPendingPods {
		log.Info("Deleting stuck pod to allow recreation with new template",
			"pod", pod.Name,
			"namespace", pod.Namespace,
			"currentRevision", currentRevision)

		if err := r.Delete(ctx, &pod); err != nil {
			log.Error(err, "Failed to delete stuck pod", "pod", pod.Name)
			continue
		}
	}

	return nil
}

// getPodsForInstanceSet returns all Pods owned by the given InstanceSet
func (r *InstanceSetReconciler) getPodsForInstanceSet(ctx context.Context, instanceSet *appsv1alpha1.InstanceSet) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(map[string]string{
		appsv1alpha1.InstanceSetNameLabel: instanceSet.Name,
	})

	if err := r.List(ctx, podList,
		&client.ListOptions{
			Namespace:     instanceSet.Namespace,
			LabelSelector: labelSelector,
		}); err != nil {
		return nil, err
	}

	return podList.Items, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InstanceSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.InstanceSet{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
