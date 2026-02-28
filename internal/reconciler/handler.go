/*
Copyright 2026.

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

package reconciler

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileContext holds shared state between handlers during reconciliation
type ReconcileContext[T any] struct {
	// Resource is the Kubernetes resource being reconciled
	Resource T

	// Result is the reconciliation result that can be modified by handlers
	Result ctrl.Result

	// Data is shared data between handlers in the chain
	Data map[string]any
}

// Handler processes a single step in the reconciliation chain
type Handler[T any] interface {
	// Handle processes the reconciliation step.
	// If an error is returned, the chain stops and the error is propagated.
	// If Result.RequeueAfter is set, the chain stops and the result is propagated.
	Handle(ctx context.Context, rc *ReconcileContext[T]) error
}

// Chain executes handlers in sequence
type Chain[T any] struct {
	handlers []Handler[T]
}

// NewChain creates a new handler chain with the given handlers
func NewChain[T any](handlers ...Handler[T]) *Chain[T] {
	return &Chain[T]{handlers: handlers}
}

// Execute runs all handlers in sequence until one errors or requests requeue
func (c *Chain[T]) Execute(ctx context.Context, rc *ReconcileContext[T]) error {
	for _, h := range c.handlers {
		if err := h.Handle(ctx, rc); err != nil {
			return err
		}
		// Short-circuit if a handler requested a delayed requeue
		if rc.Result.RequeueAfter > 0 {
			return nil
		}
	}
	return nil
}

// HandlerFunc is a function adapter for Handler interface
type HandlerFunc[T any] func(ctx context.Context, rc *ReconcileContext[T]) error

// Handle implements Handler interface
func (f HandlerFunc[T]) Handle(ctx context.Context, rc *ReconcileContext[T]) error {
	return f(ctx, rc)
}
