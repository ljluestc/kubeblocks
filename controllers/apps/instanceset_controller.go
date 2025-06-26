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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

// Reconciler reconciles an InstanceSet object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for InstanceSet
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var instanceSet workloads.InstanceSet
	if err := r.Get(ctx, req.NamespacedName, &instanceSet); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	currentRevision := instanceSet.Status.LatestRevision
	if currentRevision == nil {
		logger.Error(fmt.Errorf("latest revision not set"), "Requeueing")
		return ctrl.Result{Requeue: true}, nil
	}

	podList := &v1.PodList{}
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/instance": instanceSet.Name,
		},
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create selector: %w", err)
	}
	if err := r.List(ctx, podList, &client.ListOptions{
		Namespace:     instanceSet.Namespace,
		LabelSelector: selector,
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list pods: %w", err)
	}

	// Delete Pending Pods with outdated template
	for _, pod := range podList.Items {
		if pod.Status.Phase == v1.PodPending {
			podRevision, ok := pod.Labels["app.kubernetes.io/revision"]
			if !ok || podRevision != currentRevision.Name {
				logger.Info("Deleting Pending Pod with outdated revision",
					"Pod", pod.Name, "Revision", podRevision, "CurrentRevision", currentRevision.Name)
				if err := r.Delete(ctx, &pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to delete pod %s: %w", pod.Name, err)
				}
			}
		}
	}

	if err := r.ensureReplicas(ctx, &instanceSet, currentRevision.Name); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure replicas: %w", err)
	}

	return ctrl.Result{}, nil
}

// ensureReplicas maintains the desired number of Pods with the current template
func (r *Reconciler) ensureReplicas(ctx context.Context, instanceSet *workloads.InstanceSet, revision string) error {
	desiredReplicas := *instanceSet.Spec.Replicas
	podList := &v1.PodList{}
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/instance": instanceSet.Name,
			"app.kubernetes.io/revision": revision,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create selector: %w", err)
	}
	if err := r.List(ctx, podList, &client.ListOptions{
		Namespace:     instanceSet.Namespace,
		LabelSelector: selector,
	}); err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	currentReplicas := int32(len(podList.Items))
	if currentReplicas < desiredReplicas {
		for i := currentReplicas; i < desiredReplicas; i++ {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: fmt.Sprintf("%s-", instanceSet.Name),
					Namespace:    instanceSet.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": instanceSet.Name,
						"app.kubernetes.io/revision": revision,
					},
				},
				Spec: instanceSet.Spec.Template.Spec,
			}
			if err := controllerutil.SetControllerReference(instanceSet, pod, r.Scheme); err != nil {
				return fmt.Errorf("failed to set controller reference: %w", err)
			}
			if err := r.Create(ctx, pod); err != nil {
				return fmt.Errorf("failed to create pod: %w", err)
			}
		}
	} else if currentReplicas > desiredReplicas {
		for _, pod := range podList.Items[:currentReplicas-desiredReplicas] {
			if err := r.Delete(ctx, &pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
				return fmt.Errorf("failed to delete excess pod %s: %w", pod.Name, err)
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workloads.InstanceSet{}).
		Owns(&v1.Pod{}).
		Complete(r)
}
