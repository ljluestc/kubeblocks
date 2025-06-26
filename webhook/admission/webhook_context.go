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

package admission

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// WebhookFuncAdapter creates an adapter that converts newer style webhook handlers
// (which take a context.Context) to be compatible with older handlers
type WebhookFuncAdapter struct {
	Handler interface{}
}

// WithContextAdapter adapts a handler function that expects context to one that doesn't
func WithContextAdapter(ctx context.Context, obj interface{}) interface{} {
	// Check the type of the handler and adapt if needed
	switch fn := obj.(type) {
	case func(context.Context, admission.Request) admission.Response:
		return func(req admission.Request) admission.Response {
			return fn(ctx, req)
		}
	default:
		// Return the original handler if it doesn't match any pattern we need to adapt
		return obj
	}
}

// AdaptWebhookForV21 ensures webhook handlers are compatible with controller-runtime v0.21.0
func AdaptWebhookForV21(handler interface{}) interface{} {
	// Currently just passes through, but can be extended if specific adaptations are needed
	return handler
}
