/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package controllerutil

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// Function type for reconcilers that don't use context
type LegacyReconcileFunc func(req ctrl.Request) (ctrl.Result, error)

// ContextReconcileWrapper wraps a reconcile function to include context
type ContextReconcileWrapper struct {
	Fn LegacyReconcileFunc
}

// Reconcile implements the Reconciler interface
func (w *ContextReconcileWrapper) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return w.Fn(req)
}

// NewControllerManagedBy creates a controller builder that properly handles API version differences
func NewControllerManagedBy(mgr ctrl.Manager) *ctrl.Builder {
	return ctrl.NewControllerManagedBy(mgr)
}

// Complete implements the Controller interface with backwards compatibility
func Complete(r interface{}, mgr ctrl.Manager, options controller.Options) error {
	builder := ctrl.NewControllerManagedBy(mgr)
	return builder.Complete(r)
}
