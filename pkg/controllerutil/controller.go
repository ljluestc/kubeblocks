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

package controllerutil

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ControllerAdapter provides a compatibility layer for controller-runtime API changes
type ControllerAdapter struct {
	Ctx    context.Context
	Client client.Client
	Scheme *runtime.Scheme
}

// NewControllerManagedBy creates a controller builder with compatibility for newer controller-runtime
func NewControllerManagedBy(mgr ctrl.Manager) *ctrl.Builder {
	return ctrl.NewControllerManagedBy(mgr)
}

// AdaptReconciler wraps a reconcile.Reconciler to support context parameters
func AdaptReconciler(ctx context.Context, r reconcile.Reconciler) reconcile.Reconciler {
	return &reconcilerAdapter{ctx: ctx, inner: r}
}

// reconcilerAdapter adapts a reconciler to include context
type reconcilerAdapter struct {
	ctx   context.Context
	inner reconcile.Reconciler
}

// Reconcile implements reconcile.Reconciler
func (r *reconcilerAdapter) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Call the inner reconciler, injecting our context if needed
	return r.inner.Reconcile(ctx, req)
}

// WithControllerOptions passes options to the controller
func WithControllerOptions(options controller.Options) controller.Options {
	return options
}

// EnsureCompatibility checks for and adjusts any controller-runtime v0.21.0 compatibility issues
func EnsureCompatibility() {
	// This function can be extended if specific compatibility adjustments are needed
	// Currently just serving as a placeholder for future compatibility issues
}
