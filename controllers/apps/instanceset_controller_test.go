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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestInstanceSetReconciler_Reconcile(t *testing.T) {
	// Setup
	s := scheme.Scheme
	s.AddKnownTypes(appsv1alpha1.GroupVersion, &appsv1alpha1.InstanceSet{})

	instanceSet := &appsv1alpha1.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-instanceset",
			Namespace: "default",
		},
		Status: appsv1alpha1.InstanceSetStatus{
			CurrentRevision: "revision-1",
		},
	}

	// Create a fake client
	client := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(instanceSet).
		Build()

	// Create a InstanceSetReconciler object with the scheme and fake client
	r := &InstanceSetReconciler{
		Client: client,
		Scheme: s,
	}

	// Mock request to simulate Reconcile() being called on the controller
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-instanceset",
			Namespace: "default",
		},
	}

	// Call Reconcile
	_, err := r.Reconcile(context.Background(), req)
	assert.NoError(t, err)
}

func TestInstanceSetReconciler_handleStuckPods(t *testing.T) {
	// Setup
	instanceSet := &appsv1alpha1.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-instanceset",
			Namespace: "default",
		},
		Status: appsv1alpha1.InstanceSetStatus{
			CurrentRevision: "revision-2",
		},
	}

	stuckPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stuck-pod",
			Namespace: "default",
			Labels: map[string]string{
				appsv1alpha1.InstanceSetNameLabel:     "test-instanceset",
				appsv1alpha1.InstanceSetRevisionLabel: "revision-1",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
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
	}

	normalPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "normal-pod",
			Namespace: "default",
			Labels: map[string]string{
				appsv1alpha1.InstanceSetNameLabel:     "test-instanceset",
				appsv1alpha1.InstanceSetRevisionLabel: "revision-2",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	// Create fake client with instanceset and pods
	s := scheme.Scheme
	s.AddKnownTypes(appsv1alpha1.GroupVersion, &appsv1alpha1.InstanceSet{})

	client := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(instanceSet, stuckPod, normalPod).
		Build()

	// Create reconciler
	reconciler := &InstanceSetReconciler{
		Client: client,
		Scheme: s,
	}

	// Test
	ctx := context.Background()
	err := reconciler.handleStuckPods(ctx, instanceSet)

	// Assertions
	assert.NoError(t, err)

	// Verify the stuck pod was deleted
	err = client.Get(ctx, types.NamespacedName{Name: "stuck-pod", Namespace: "default"}, &corev1.Pod{})
	assert.True(t, errors.IsNotFound(err), "Stuck pod should be deleted")

	// Verify the normal pod still exists
	var pod corev1.Pod
	err = client.Get(ctx, types.NamespacedName{Name: "normal-pod", Namespace: "default"}, &pod)
	assert.NoError(t, err, "Normal pod should still exist")
	assert.Equal(t, corev1.PodRunning, pod.Status.Phase)
}
