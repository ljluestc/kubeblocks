package instanceset
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

func TestReconciler_ReconcileWithStuckPods(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = workloadsv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// Create a fake InstanceSet
	instanceSet := &workloadsv1alpha1.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-instanceset",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
		Spec: workloadsv1alpha1.InstanceSetSpec{
			Replicas: int32Ptr(3),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test-image",
						},
					},
				},
			},
		},
		Status: workloadsv1alpha1.InstanceSetStatus{
			CurrentRevision: "rev-2",
		},
	}

	// Create pods with different statuses
	runningPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-running-pod",
			Namespace: "default",
			Labels: map[string]string{
				workloadsv1alpha1.InstanceSetNameLabel:     "test-instanceset",
				workloadsv1alpha1.InstanceSetRevisionLabel: "rev-2", // current revision
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: workloadsv1alpha1.SchemeGroupVersion.String(),
					Kind:       "InstanceSet",
					Name:       instanceSet.Name,
					UID:        instanceSet.UID,
					Controller: boolPtr(true),
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	// Pending pod with outdated revision
	pendingOutdatedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pending-outdated-pod",
			Namespace: "default",
			Labels: map[string]string{
				workloadsv1alpha1.InstanceSetNameLabel:     "test-instanceset",
				workloadsv1alpha1.InstanceSetRevisionLabel: "rev-1", // outdated revision
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)), // Created 10 minutes ago
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: workloadsv1alpha1.SchemeGroupVersion.String(),
					Kind:       "InstanceSet",
					Name:       instanceSet.Name,
					UID:        instanceSet.UID,
					Controller: boolPtr(true),
				},
			},
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
	}

	// Pending pod with current revision but stuck
	pendingStuckPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pending-stuck-pod",
			Namespace: "default",
			Labels: map[string]string{
				workloadsv1alpha1.InstanceSetNameLabel:     "test-instanceset",
				workloadsv1alpha1.InstanceSetRevisionLabel: "rev-2", // current revision
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)), // Created 10 minutes ago
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: workloadsv1alpha1.SchemeGroupVersion.String(),
					Kind:       "InstanceSet",
					Name:       instanceSet.Name,
					UID:        instanceSet.UID,
					Controller: boolPtr(true),
				},
			},
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
	}

	// Setup fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instanceSet, runningPod, pendingOutdatedPod, pendingStuckPod).
		Build()

	// Create a reconciler with the fake client
	reconciler := &InstanceSetReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: &record.FakeRecorder{},
	}

	// Call reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      instanceSet.Name,
			Namespace: instanceSet.Namespace,
		},
	}
	_, err := reconciler.Reconcile(context.Background(), req)
	assert.NoError(t, err)

	// Verify that the outdated pending pod was deleted
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Namespace: "default",
		Name:      "test-pending-outdated-pod",
	}, &corev1.Pod{})
	assert.Error(t, err)
	assert.True(t, client.IgnoreNotFound(err) == nil, "Expected pod to be deleted")

	// Verify that the stuck pending pod was deleted
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Namespace: "default",
		Name:      "test-pending-stuck-pod",
	}, &corev1.Pod{})
	assert.Error(t, err)
	assert.True(t, client.IgnoreNotFound(err) == nil, "Expected pod to be deleted")

	// Verify that the running pod was not deleted
	var runningPodCheck corev1.Pod
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Namespace: "default",
		Name:      "test-running-pod",
	}, &runningPodCheck)
	assert.NoError(t, err)
	assert.Equal(t, corev1.PodRunning, runningPodCheck.Status.Phase)
}

func TestInstanceSetReconciler_RecoverStuckPods(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = workloadsv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// Create a test InstanceSet
	instanceSet := &workloadsv1alpha1.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-instanceset",
			Namespace: "default",
		},
		Status: workloadsv1alpha1.InstanceSetStatus{
			CurrentRevision: "rev-current",
		},
	}

	// Create a pod with outdated revision that's stuck in Pending state
	outdatedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-outdated-pod",
			Namespace: "default",
			Labels: map[string]string{
				workloadsv1alpha1.InstanceSetNameLabel:     instanceSet.Name,
				workloadsv1alpha1.InstanceSetRevisionLabel: "rev-old",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: workloadsv1alpha1.GroupVersion.String(),
					Kind:       "InstanceSet",
					Name:       instanceSet.Name,
					Controller: func() *bool { b := true; return &b }(),
				},
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	// Create a pod with current revision but stuck due to scheduling issues
	stuckPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-stuck-pod",
			Namespace: "default",
			Labels: map[string]string{
				workloadsv1alpha1.InstanceSetNameLabel:     instanceSet.Name,
				workloadsv1alpha1.InstanceSetRevisionLabel: "rev-current",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: workloadsv1alpha1.GroupVersion.String(),
					Kind:       "InstanceSet",
					Name:       instanceSet.Name,
					Controller: func() *bool { b := true; return &b }(),
				},
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
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
	}

	// Create a healthy running pod
	runningPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-running-pod",
			Namespace: "default",
			Labels: map[string]string{
				workloadsv1alpha1.InstanceSetNameLabel:     instanceSet.Name,
				workloadsv1alpha1.InstanceSetRevisionLabel: "rev-current",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: workloadsv1alpha1.GroupVersion.String(),
					Kind:       "InstanceSet",
					Name:       instanceSet.Name,
					Controller: func() *bool { b := true; return &b }(),
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	// Create the fake client with our objects
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instanceSet, outdatedPod, stuckPod, runningPod).
		Build()

	// Create the reconciler
	reconciler := &InstanceSetReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: &record.FakeRecorder{},
	}

	// Call recoverStuckPods directly
	err := reconciler.recoverStuckPods(context.Background(), instanceSet)
	assert.NoError(t, err)

	// Verify the outdated pod was deleted
	var pod corev1.Pod
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      outdatedPod.Name,
		Namespace: outdatedPod.Namespace,
	}, &pod)
	assert.Error(t, err)
	assert.True(t, client.IgnoreNotFound(err) == nil)

	// Verify the stuck pod was deleted
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      stuckPod.Name,
		Namespace: stuckPod.Namespace,
	}, &pod)
	assert.Error(t, err)
	assert.True(t, client.IgnoreNotFound(err) == nil)

	// Verify the running pod was not deleted
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      runningPod.Name,
		Namespace: runningPod.Namespace,
	}, &pod)
	assert.NoError(t, err)
	assert.Equal(t, corev1.PodRunning, pod.Status.Phase)
}

func int32Ptr(i int32) *int32 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/oam-dev/kubevela/apis/core.oam.dev/v1beta1"
	"github.com/oam-dev/kubevela/pkg/testing"
)

func TestInstanceSetController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceSet Controller Suite")
}

var _ = Describe("InstanceSet Controller", func() {
	var (
		ctx        = context.Background()
		testEnv    *envtest.Environment
		k8sClient  client.Client
		namespace  = "test-instanceset"
		reconciler *Reconciler
	)

	BeforeEach(func() {
		var err error
		testEnv, err = testing.SetupEnvTest([]string{"../../../../../charts/vela-core/crds"})
		Expect(err).NotTo(HaveOccurred())
		k8sClient, err = testing.GetClientFromEnv(testEnv, nil)
		Expect(err).NotTo(HaveOccurred())

		reconciler = &Reconciler{
			Client: k8sClient,
			Scheme: testEnv.Scheme,
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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

func TestInstanceSetReconciler_RecoverStuckPods(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = workloadsv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// Create a test InstanceSet
	instanceSet := &workloadsv1alpha1.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-instanceset",
			Namespace: "default",
		},
		Status: workloadsv1alpha1.InstanceSetStatus{
			CurrentRevision: "rev-current",
		},
	}

	// Create a pod with outdated revision that's stuck in Pending state
	outdatedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-outdated-pod",
			Namespace: "default",
			Labels: map[string]string{
				workloadsv1alpha1.InstanceSetNameLabel:     instanceSet.Name,
				workloadsv1alpha1.InstanceSetRevisionLabel: "rev-old",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: workloadsv1alpha1.GroupVersion.String(),
					Kind:       "InstanceSet",
					Name:       instanceSet.Name,
					Controller: func() *bool { b := true; return &b }(),
				},
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	// Create a pod with current revision but stuck due to scheduling issues
	stuckPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-stuck-pod",
			Namespace: "default",
			Labels: map[string]string{
				workloadsv1alpha1.InstanceSetNameLabel:     instanceSet.Name,
				workloadsv1alpha1.InstanceSetRevisionLabel: "rev-current",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: workloadsv1alpha1.GroupVersion.String(),
					Kind:       "InstanceSet",
					Name:       instanceSet.Name,
					Controller: func() *bool { b := true; return &b }(),
				},
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
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
	}

	// Create a healthy running pod
	runningPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-running-pod",
			Namespace: "default",
			Labels: map[string]string{
				workloadsv1alpha1.InstanceSetNameLabel:     instanceSet.Name,
				workloadsv1alpha1.InstanceSetRevisionLabel: "rev-current",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: workloadsv1alpha1.GroupVersion.String(),
					Kind:       "InstanceSet",
					Name:       instanceSet.Name,
					Controller: func() *bool { b := true; return &b }(),
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
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

func TestInstanceSetReconciler_Reconcile(t *testing.T) {
	// Create a fake client with an InstanceSet
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = workloadsv1alpha1.AddToScheme(scheme)

	instanceSet := &workloadsv1alpha1.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-instanceset",
			Namespace: "default",
		},
		Spec: workloadsv1alpha1.InstanceSetSpec{
			Replicas: 3,
		},
		Status: workloadsv1alpha1.InstanceSetStatus{
			CurrentRevision: "rev1",
		},
	}

	pod1 := createTestPod("pod1", "rev1", corev1.PodRunning)
	pod1.Labels = map[string]string{
		workloadsv1alpha1.InstanceSetNameLabel: "test-instanceset",
	}
	pod1.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instanceSet, pod1).Build()

	// Create reconciler
	recorder := record.NewFakeRecorder(10)
	r := &InstanceSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: recorder,
	}

	// Test reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-instanceset",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	ctx = log.IntoContext(ctx, log.Log)
	result, err := r.Reconcile(ctx, req)

	// Verify results
	assert.NoError(t, err, "Reconcile should not return an error")
	assert.Equal(t, ctrl.Result{}, result, "Reconcile should return an empty Result")

	// Verify instanceset status was updated
	updatedInstanceSet := &workloadsv1alpha1.InstanceSet{}
	err = client.Get(ctx, types.NamespacedName{
		Namespace: instanceSet.Namespace,
		Name:      instanceSet.Name,
	}, updatedInstanceSet)
	assert.NoError(t, err, "Should be able to get updated InstanceSet")
	assert.Equal(t, 1, updatedInstanceSet.Status.ReadyReplicas, "Should have 1 ready replica")
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
	// Create the fake client with our objects
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instanceSet, outdatedPod, stuckPod, runningPod).
		Build()

	// Create the reconciler
	reconciler := &InstanceSetReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: &record.FakeRecorder{},
	}

	// Run reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      instanceSet.Name,
			Namespace: instanceSet.Namespace,
		},
	}
	_, err := reconciler.Reconcile(context.Background(), req)
	assert.NoError(t, err)

	// Verify the outdated pod was deleted
	var pod corev1.Pod
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      outdatedPod.Name,
		Namespace: outdatedPod.Namespace,
	}, &pod)
	assert.Error(t, err)
	assert.True(t, client.IgnoreNotFound(err) == nil)

	// Verify the stuck pod was deleted
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      stuckPod.Name,
		Namespace: stuckPod.Namespace,
	}, &pod)
	assert.Error(t, err)
	assert.True(t, client.IgnoreNotFound(err) == nil)

	// Verify the running pod was not deleted
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      runningPod.Name,
		Namespace: runningPod.Namespace,
	}, &pod)
	assert.NoError(t, err)
	assert.Equal(t, corev1.PodRunning, pod.Status.Phase)
}
		// Create test namespace
		ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		Expect(testing.CreateObject(ctx, k8sClient, ns)).To(Succeed())
	})

	AfterEach(func() {
		Expect(testEnv.Stop()).To(Succeed())
	})

	It("should recover Pending Pods when the template is updated", func() {
		// Create InstanceSet with revision1
		replicas := int32(2)
		instanceSet := &v1beta1.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-instanceset",
				Namespace: namespace,
			},
			Spec: v1beta1.InstanceSetSpec{
				Replicas: &replicas,
				Template: v1beta1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "nginx",
								Image: "nginx:1.19",
							},
						},
					},
				},
			},
			Status: v1beta1.InstanceSetStatus{
				LatestRevision: &v1beta1.Revision{
					Name: "revision1",
				},
			},
		}
		Expect(testing.CreateObject(ctx, k8sClient, instanceSet)).To(Succeed())

		// Create a Pending Pod with revision1
		pendingPod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pending-pod",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-instanceset",
					"app.kubernetes.io/revision": "revision1",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: v1beta1.SchemeGroupVersion.String(),
						Kind:       "InstanceSet",
						Name:       "test-instanceset",
						UID:        instanceSet.UID,
						Controller: &[]bool{true}[0],
					},
				},
			},
			Spec: instanceSet.Spec.Template.Spec,
			Status: v1.PodStatus{
				Phase: v1.PodPending,
				Conditions: []v1.PodCondition{
					{
						Type:    v1.PodScheduled,
						Status:  v1.ConditionFalse,
						Reason:  "Unschedulable",
						Message: "0/1 nodes are available: 1 node(s) didn't match pod anti-affinity rules.",
					},
				},
			},
		}
		Expect(testing.CreateObject(ctx, k8sClient, pendingPod)).To(Succeed())

		// Create a Running Pod with revision1
		runningPod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "running-pod",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-instanceset",
					"app.kubernetes.io/revision": "revision1",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: v1beta1.SchemeGroupVersion.String(),
						Kind:       "InstanceSet",
						Name:       "test-instanceset",
						UID:        instanceSet.UID,
						Controller: &[]bool{true}[0],
					},
				},
			},
			Spec: instanceSet.Spec.Template.Spec,
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
			},
		}
		Expect(testing.CreateObject(ctx, k8sClient, runningPod)).To(Succeed())

		// Update InstanceSet with revision2
		updatedInstanceSet := instanceSet.DeepCopy()
		updatedInstanceSet.Status.LatestRevision = &v1beta1.Revision{
			Name: "revision2",
		}
		// Update the spec to simulate a good template
		updatedInstanceSet.Spec.Template.Spec.Containers[0].Image = "nginx:1.20"
		Expect(k8sClient.Update(ctx, updatedInstanceSet)).To(Succeed())

		// Reconcile
		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      updatedInstanceSet.Name,
				Namespace: updatedInstanceSet.Namespace,
			},
		}
		result, err := reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		// Verify that Pending Pod was deleted
		pendingPodKey := types.NamespacedName{Name: "pending-pod", Namespace: namespace}
		err = k8sClient.Get(ctx, pendingPodKey, &v1.Pod{})
		Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		Expect(err).To(HaveOccurred()) // Pod should be deleted

		// Verify that Running Pod was not deleted
		runningPodKey := types.NamespacedName{Name: "running-pod", Namespace: namespace}
		foundRunningPod := &v1.Pod{}
		err = k8sClient.Get(ctx, runningPodKey, foundRunningPod)
		Expect(err).NotTo(HaveOccurred())
		Expect(foundRunningPod.Status.Phase).To(Equal(v1.PodRunning))
	})
})
