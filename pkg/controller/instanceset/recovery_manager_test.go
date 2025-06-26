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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

func TestRecoveryManager_FindOutdatedPendingPods(t *testing.T) {
	// Create a fake client with some test pods
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = workloadsv1alpha1.AddToScheme(scheme)

	// Create test pods
	pod1 := createTestPod("pod1", "rev1", corev1.PodPending)
	pod2 := createTestPod("pod2", "rev1", corev1.PodRunning)
	pod3 := createTestPod("pod3", "rev2", corev1.PodPending)

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod1, pod2, pod3).Build()

	podList := &corev1.PodList{
		Items: []corev1.Pod{*pod1, *pod2, *pod3},
	}

	// Create recovery manager
	ctx := context.Background()
	recoveryManager := NewRecoveryManager(ctx, client, podList)

	// Test finding outdated pending pods
	outdatedPods := recoveryManager.FindOutdatedPendingPods("rev1")

	// Verify results
	assert.Equal(t, 1, len(outdatedPods), "Should find 1 outdated pending pod")
	assert.Equal(t, "pod3", outdatedPods[0].Name, "Outdated pod should be pod3")
}

func TestRecoveryManager_FindStuckPendingPods(t *testing.T) {
	// Create a fake client with some test pods
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Create test pods - one stuck, one normal pending
	stuckPod := createTestPod("stuck-pod", "rev1", corev1.PodPending)
	stuckPod.Status.StartTime = &metav1.Time{Time: time.Now().Add(-10 * time.Minute)} // 10 minutes old
	stuckPod.Status.Conditions = []corev1.PodCondition{
		{
			Type:    corev1.PodScheduled,
			Status:  corev1.ConditionFalse,
			Reason:  "Unschedulable",
			Message: "0/3 nodes are available: insufficient resources",
		},
	}

	normalPod := createTestPod("normal-pod", "rev1", corev1.PodPending)
	normalPod.Status.StartTime = &metav1.Time{Time: time.Now().Add(-1 * time.Minute)} // 1 minute old

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(stuckPod, normalPod).Build()

	podList := &corev1.PodList{
		Items: []corev1.Pod{*stuckPod, *normalPod},
	}

	// Create recovery manager
	ctx := context.Background()
	recoveryManager := NewRecoveryManager(ctx, client, podList)

	// Test finding stuck pending pods
	stuckPods := recoveryManager.FindStuckPendingPods()

	// Verify results
	assert.Equal(t, 1, len(stuckPods), "Should find 1 stuck pending pod")
	assert.Equal(t, "stuck-pod", stuckPods[0].Name, "Stuck pod should be stuck-pod")
}

func TestRecoveryManager_DeletePods(t *testing.T) {
	// Create a fake client with a test pod
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	pod1 := createTestPod("pod1", "rev1", corev1.PodPending)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod1).Build()

	podList := &corev1.PodList{
		Items: []corev1.Pod{*pod1},
	}

	// Create recovery manager
	ctx := context.Background()
	recoveryManager := NewRecoveryManager(ctx, client, podList)

	// Test deleting pods
	errors := recoveryManager.DeletePods([]corev1.Pod{*pod1})

	// Verify results
	assert.Equal(t, 0, len(errors), "Should not have any errors when deleting pod")

	// Verify pod is deleted
	deletedPod := &corev1.Pod{}
	err := client.Get(ctx, types.NamespacedName{Namespace: pod1.Namespace, Name: pod1.Name}, deletedPod)
	assert.Error(t, err, "Pod should be deleted")
}
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

func TestRecoveryManager_FindOutdatedPendingPods(t *testing.T) {
	// Setup test data
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	podList := &corev1.PodList{
		Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod1",
					Labels: map[string]string{
						workloadsv1alpha1.InstanceSetRevisionLabel: "rev1",
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod2",
					Labels: map[string]string{
						workloadsv1alpha1.InstanceSetRevisionLabel: "rev2",
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod3",
					Labels: map[string]string{
						workloadsv1alpha1.InstanceSetRevisionLabel: "rev2",
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	rm := NewRecoveryManager(context.Background(), client, podList)

	// Test finding outdated pending pods
	outdatedPods := rm.FindOutdatedPendingPods("rev2")
	assert.Equal(t, 1, len(outdatedPods))
	assert.Equal(t, "pod1", outdatedPods[0].Name)
}

func TestRecoveryManager_FindStuckPendingPods(t *testing.T) {
	// Setup test data
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	stuckTime := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	freshTime := metav1.NewTime(time.Now().Add(-1 * time.Minute))

	podList := &corev1.PodList{
		Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "stuck-pod",
					CreationTimestamp: stuckTime,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionFalse,
							Reason: "Unschedulable",
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "fresh-pod",
					CreationTimestamp: freshTime,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "old-pod-not-unschedulable",
					CreationTimestamp: stuckTime,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	rm := NewRecoveryManager(context.Background(), client, podList)

	// Test finding stuck pending pods
	stuckPods := rm.FindStuckPendingPods()
	assert.Equal(t, 1, len(stuckPods))
	assert.Equal(t, "stuck-pod", stuckPods[0].Name)
}
// Helper function to create test pods
func createTestPod(name, revision string, phase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				workloadsv1alpha1.InstanceSetRevisionLabel: revision,
			},
		},
		Status: corev1.PodStatus{
			Phase: phase,
		},
	}
}
