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
	"reflect"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/golgoth31/sreportal/internal/metrics"
)

// ReconcileContext holds shared state between handlers during reconciliation.
// T is the Kubernetes resource type, D is the typed chain data shared between handlers.
type ReconcileContext[T any, D any] struct {
	// Resource is the Kubernetes resource being reconciled
	Resource T

	// Result is the reconciliation result that can be modified by handlers
	Result ctrl.Result

	// Data is typed shared data between handlers in the chain
	Data D
}

// Handler processes a single step in the reconciliation chain
type Handler[T any, D any] interface {
	// Handle processes the reconciliation step.
	// If an error is returned, the chain stops and the error is propagated.
	// If Result.RequeueAfter is set, the chain stops and the result is propagated.
	Handle(ctx context.Context, rc *ReconcileContext[T, D]) error
}

// Chain executes handlers in sequence
type Chain[T any, D any] struct {
	controller string
	handlers   []Handler[T, D]
}

// NewChain creates a new handler chain. controller is the controller name used
// as the `controller` label in per-handler ReconcileDuration observations
// (e.g. "imageinventory"). Use an empty string to skip per-handler timing.
func NewChain[T any, D any](controller string, handlers ...Handler[T, D]) *Chain[T, D] {
	return &Chain[T, D]{controller: controller, handlers: handlers}
}

// Execute runs all handlers in sequence until one errors or requests requeue.
// When the chain has a controller name set, each handler's duration is observed
// on metrics.ReconcileDuration with handler=<TypeName>.
func (c *Chain[T, D]) Execute(ctx context.Context, rc *ReconcileContext[T, D]) error {
	for _, h := range c.handlers {
		start := time.Now()
		err := h.Handle(ctx, rc)
		c.observe(h, start)
		if err != nil {
			return err
		}
		// Short-circuit if a handler requested a delayed requeue
		if rc.Result.RequeueAfter > 0 {
			return nil
		}
	}
	return nil
}

// observe records the duration of a single handler step. Skipped when the
// chain was built without a controller name (e.g. unit tests of Execute).
func (c *Chain[T, D]) observe(h Handler[T, D], start time.Time) {
	if c.controller == "" {
		return
	}
	metrics.ReconcileDuration.
		WithLabelValues(c.controller, handlerName(h)).
		Observe(time.Since(start).Seconds())
}

// handlerName returns the Go type name of a handler (e.g. "*ScanWorkloadsHandler"
// becomes "ScanWorkloadsHandler"). Anonymous types (HandlerFunc) fall back to
// "anonymous" to keep label cardinality bounded.
func handlerName(h any) string {
	t := reflect.TypeOf(h)
	if t == nil {
		return "anonymous"
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	name := t.Name()
	if name == "" {
		return "anonymous"
	}
	return name
}

// HandlerFunc is a function adapter for Handler interface
type HandlerFunc[T any, D any] func(ctx context.Context, rc *ReconcileContext[T, D]) error

// Handle implements Handler interface
func (f HandlerFunc[T, D]) Handle(ctx context.Context, rc *ReconcileContext[T, D]) error {
	return f(ctx, rc)
}
