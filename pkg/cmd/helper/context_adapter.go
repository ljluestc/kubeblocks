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

package helper

import (
	"context"
)

// ContextAdapter provides a compatibility layer for functions that require context
type ContextAdapter struct {
	Ctx context.Context
}

// NewContextAdapter creates a new context adapter
func NewContextAdapter() *ContextAdapter {
	return &ContextAdapter{
		Ctx: context.Background(),
	}
}

// WithContext returns a copy of the adapter with the given context
func (a *ContextAdapter) WithContext(ctx context.Context) *ContextAdapter {
	return &ContextAdapter{
		Ctx: ctx,
	}
}

// AdaptFn adapts a function to include context
func (a *ContextAdapter) AdaptFn(fn interface{}) interface{} {
	// Adapt different function signatures as needed
	switch f := fn.(type) {
	case func(interface{}):
		return func(ctx context.Context, obj interface{}) {
			f(obj)
		}
	default:
		return fn
	}
}
