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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

func TestFindOutdatedPendingPods(t *testing.T) {
	// Create a fake client
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Create test pods
	podList := &corev1.PodList{
		Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-current",
					Namespace: "default",
					Labels: map[string]string{
						workloadsv1alpha1.InstanceSetRevisionLabel: "rev-current",
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-outdated",
					Namespace: "default",
					Labels: map[string]string{
						workloadsv1alpha1.InstanceSetRevisionLabel: "rev-old",
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-running",
					Namespace: "default",
					Labels: map[string]string{
						workloadsv1alpha1.InstanceSetRevisionLabel: "rev-old",
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	ctx := context.Background()
	
	rm := NewRecoveryManager(ctx, client, podList)
	
	// Test finding outdated pending pods
	outdatedPods := rm.FindOutdatedPendingPods("rev-current")
	
	// Should find only one outdated pending pod
	assert.Equal(t, 1, len(outdatedPods))
	assert.Equal(t, "pod-outdated", outdatedPods[0].Name)
}

func TestFindStuckPendingPods(t *testing.T) {
	// Create a fake client
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Create timestamps
	oldTime := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	newTime := metav1.NewTime(time.Now().Add(-1 * time.Minute))
	
	// Create test pods
	podList := &corev1.PodList{
		Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-stuck",
					Namespace: "default",
					CreationTimestamp: oldTime,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{
						{
							Type:    corev1.PodScheduled,
							Status:  corev1.ConditionFalse,
							Reason:  "Unschedulable",
							Message: "0/3 nodes are available: 3 node(s) didn't match pod anti-affinity rules.",
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-new-pending",
					Namespace: "default",
					CreationTimestamp: newTime,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-running",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	ctx := context.Background()
	
	rm := NewRecoveryManager(ctx, client, podList)
	
	// Test finding stuck pending pods
	stuckPods := rm.FindStuckPendingPods()
	
	// Should find only one stuck pending pod
	assert.Equal(t, 1, len(stuckPods))
	assert.Equal(t, "pod-stuck", stuckPods[0].Name)
}
