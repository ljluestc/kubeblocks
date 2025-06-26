/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd
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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

const (
	// StuckPodTimeout is the time duration after which a pending pod is considered stuck
	StuckPodTimeout = 5 * time.Minute
)

// RecoveryManager handles recovery operations for InstanceSets
type RecoveryManager struct {
	client  client.Client
	logger  *log.DelegatingLogger
	ctx     context.Context
	podList *corev1.PodList
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(ctx context.Context, c client.Client, podList *corev1.PodList) *RecoveryManager {
	return &RecoveryManager{
		client:  c,
		logger:  log.FromContext(ctx),
		ctx:     ctx,
		podList: podList,
	}
}

// FindOutdatedPendingPods identifies pending pods with outdated revision
func (r *RecoveryManager) FindOutdatedPendingPods(currentRevision string) []corev1.Pod {
	outdatedPods := make([]corev1.Pod, 0)
	
	for i := range r.podList.Items {
		pod := &r.podList.Items[i]
		if pod.Status.Phase == corev1.PodPending {
			podRevision := pod.Labels[workloadsv1alpha1.InstanceSetRevisionLabel]
			if podRevision != currentRevision {
				outdatedPods = append(outdatedPods, *pod)
			}
		}
	}
	
	return outdatedPods
}

// FindStuckPendingPods identifies pods that are stuck in pending state
func (r *RecoveryManager) FindStuckPendingPods() []corev1.Pod {
	stuckPods := make([]corev1.Pod, 0)
	
	for i := range r.podList.Items {
		pod := &r.podList.Items[i]
		if pod.Status.Phase == corev1.PodPending {
			// Check if pod has been pending for too long (more than 5 minutes)
			pendingTooLong := false
			if pod.Status.StartTime != nil {
				pendingDuration := time.Since(pod.Status.StartTime.Time)
				pendingTooLong = pendingDuration > StuckPodTimeout
			} else if !pod.CreationTimestamp.IsZero() {
				pendingDuration := time.Since(pod.CreationTimestamp.Time)
				pendingTooLong = pendingDuration > StuckPodTimeout
			}

			if pendingTooLong {
				// Check for scheduling issues in pod conditions
				for _, condition := range pod.Status.Conditions {
					if condition.Type == corev1.PodScheduled &&
						condition.Status == corev1.ConditionFalse &&
						condition.Reason == "Unschedulable" {
						stuckPods = append(stuckPods, *pod)
						break
					}
				}
			}
		}
	}
	
	return stuckPods
}

// DeletePods deletes the given pods and returns any errors encountered
func (r *RecoveryManager) DeletePods(pods []corev1.Pod) []error {
	var errors []error
	
	for _, pod := range pods {
		if err := r.client.Delete(r.ctx, &pod); err != nil {
			r.logger.Error(err, "Failed to delete pod", "pod", pod.Name)
			errors = append(errors, fmt.Errorf("failed to delete pod %s: %w", pod.Name, err))
		} else {
			r.logger.Info("Successfully deleted pod", "pod", pod.Name)
		}
	}
	
	return errors
}
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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

const (
	// StuckPodTimeout is the time duration after which a pending pod is considered stuck
	StuckPodTimeout = 5 * time.Minute
)

// RecoveryManager handles recovery operations for InstanceSets
type RecoveryManager struct {
	client  client.Client
	logger  *log.DelegatingLogger
	ctx     context.Context
	podList *corev1.PodList
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(ctx context.Context, c client.Client, podList *corev1.PodList) *RecoveryManager {
	return &RecoveryManager{
		client:  c,
		logger:  log.FromContext(ctx),
		ctx:     ctx,
		podList: podList,
	}
}

// FindOutdatedPendingPods identifies pending pods with outdated revision
func (r *RecoveryManager) FindOutdatedPendingPods(currentRevision string) []corev1.Pod {
	outdatedPods := make([]corev1.Pod, 0)

	for i := range r.podList.Items {
		pod := &r.podList.Items[i]
		if pod.Status.Phase == corev1.PodPending {
			podRevision := pod.Labels[workloadsv1alpha1.InstanceSetRevisionLabel]
			if podRevision != currentRevision {
				outdatedPods = append(outdatedPods, *pod)
			}
		}
	}

	return outdatedPods
}

// FindStuckPendingPods identifies pods that are stuck in pending state
func (r *RecoveryManager) FindStuckPendingPods() []corev1.Pod {
	stuckPods := make([]corev1.Pod, 0)

	for i := range r.podList.Items {
		pod := &r.podList.Items[i]
		if pod.Status.Phase == corev1.PodPending {
			// Check if pod has been pending for too long (more than 5 minutes)
			pendingTooLong := false
			if pod.Status.StartTime != nil {
				pendingDuration := time.Since(pod.Status.StartTime.Time)
				pendingTooLong = pendingDuration > StuckPodTimeout
			} else if !pod.CreationTimestamp.IsZero() {
				pendingDuration := time.Since(pod.CreationTimestamp.Time)
				pendingTooLong = pendingDuration > StuckPodTimeout
			}

			if pendingTooLong {
				// Check for scheduling issues in pod conditions
				for _, condition := range pod.Status.Conditions {
					if condition.Type == corev1.PodScheduled &&
						condition.Status == corev1.ConditionFalse &&
						condition.Reason == "Unschedulable" {
						stuckPods = append(stuckPods, *pod)
						break
					}
				}
			}
		}
	}

	return stuckPods
}

// DeletePods deletes the given pods and returns any errors encountered
func (r *RecoveryManager) DeletePods(pods []corev1.Pod) []error {
	var errors []error

	for _, pod := range pods {
		if err := r.client.Delete(r.ctx, &pod); err != nil {
			r.logger.Error(err, "Failed to delete pod", "pod", pod.Name)
			errors = append(errors, fmt.Errorf("failed to delete pod %s: %w", pod.Name, err))
		} else {
			r.logger.Info("Successfully deleted pod", "pod", pod.Name)
		}
	}

	return errors
}
