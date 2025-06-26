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
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
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
		testEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{"../../../config/crd/bases"},
		}
		cfg, err := testEnv.Start()
		Expect(err).NotTo(HaveOccurred())
		k8sClient, err = client.New(cfg, client.Options{Scheme: testEnv.Scheme})
		Expect(err).NotTo(HaveOccurred())

		reconciler = &Reconciler{
			Client: k8sClient,
			Scheme: testEnv.Scheme,
		}

		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	})

	AfterEach(func() {
		Expect(testEnv.Stop()).To(Succeed())
	})

	It("should recover Pending Pods with updated template", func() {
		replicas := int32(3)
		instanceSet := workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-instanceset",
				Namespace: namespace,
			},
			Spec: workloads.InstanceSetSpec{
				Replicas: &replicas,
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{"app": "nonexistent"},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "app",
								Image: "nginx:latest",
							},
						},
					},
				},
			},
			Status: workloads.InstanceSetStatus{
				LatestRevision: &workloads.Revision{
					Name: "revision1",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &instanceSet)).To(Succeed())

		for i := 0; i < 2; i++ {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("bad-pod-%d", i),
					Namespace: namespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": "test-instanceset",
						"app.kubernetes.io/revision": "revision1",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: workloads.SchemeGroupVersion.String(),
							Kind:       "InstanceSet",
							Name:       instanceSet.Name,
							UID:        instanceSet.UID,
							Controller: ptrTo(true),
						},
					},
				},
				Spec:   instanceSet.Spec.Template.Spec,
				Status: corev1.PodStatus{Phase: corev1.PodPending},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		}

		runningPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "running-pod",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-instanceset",
					"app.kubernetes.io/revision": "revision1",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: workloads.SchemeGroupVersion.String(),
						Kind:       "InstanceSet",
						Name:       instanceSet.Name,
						UID:        instanceSet.UID,
						Controller: ptrTo(true),
					},
				},
			},
			Spec:   instanceSet.Spec.Template.Spec,
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}
		Expect(k8sClient.Create(ctx, runningPod)).To(Succeed())

		updatedInstanceSet := instanceSet.DeepCopy()
		updatedInstanceSet.Spec.Template.Spec.Affinity = nil
		updatedInstanceSet.Status.LatestRevision = &workloads.Revision{Name: "revision2"}
		Expect(k8sClient.Update(ctx, updatedInstanceSet)).To(Succeed())

		req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "test-instanceset", Namespace: namespace}}
		_, err := reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() int {
			podList := &corev1.PodList{}
			err := k8sClient.List(ctx, podList, &client.ListOptions{Namespace: namespace})
			if err != nil {
				return 0
			}
			runningCount := 0
			for _, pod := range podList.Items {
				if pod.Status.Phase == corev1.PodRunning && pod.Labels["app.kubernetes.io/revision"] == "revision2" {
					runningCount++
				}
			}
			return runningCount
		}, 10*time.Second, 1*time.Second).Should(Equal(3), "All replicas should be Running with revision2")
	})
})

func ptrTo[T any](v T) *T {
	return &v
}
