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

package reconciler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/reconciler"
)

func TestChain_Execute_StopsOnError(t *testing.T) {
	var secondCalled bool
	chain := reconciler.NewChain[*struct{}](
		reconciler.HandlerFunc[*struct{}](func(_ context.Context, _ *reconciler.ReconcileContext[*struct{}]) error {
			return errors.New("first handler failed")
		}),
		reconciler.HandlerFunc[*struct{}](func(_ context.Context, _ *reconciler.ReconcileContext[*struct{}]) error {
			secondCalled = true
			return nil
		}),
	)

	rc := &reconciler.ReconcileContext[*struct{}]{Data: make(map[string]any)}
	err := chain.Execute(context.Background(), rc)

	require.Error(t, err)
	assert.False(t, secondCalled, "second handler must not run after an error")
}

func TestChain_Execute_StopsOnRequeueAfter(t *testing.T) {
	var secondCalled bool
	chain := reconciler.NewChain[*struct{}](
		reconciler.HandlerFunc[*struct{}](func(_ context.Context, rc *reconciler.ReconcileContext[*struct{}]) error {
			rc.Result.RequeueAfter = 5 * time.Second
			return nil
		}),
		reconciler.HandlerFunc[*struct{}](func(_ context.Context, _ *reconciler.ReconcileContext[*struct{}]) error {
			secondCalled = true
			return nil
		}),
	)

	rc := &reconciler.ReconcileContext[*struct{}]{Data: make(map[string]any)}
	err := chain.Execute(context.Background(), rc)

	require.NoError(t, err)
	assert.False(t, secondCalled, "second handler must not run after RequeueAfter is set")
}

func TestChain_Execute_RunsAllHandlers_WhenNoShortCircuit(t *testing.T) {
	var calls []int
	chain := reconciler.NewChain[*struct{}](
		reconciler.HandlerFunc[*struct{}](func(_ context.Context, _ *reconciler.ReconcileContext[*struct{}]) error {
			calls = append(calls, 1)
			return nil
		}),
		reconciler.HandlerFunc[*struct{}](func(_ context.Context, _ *reconciler.ReconcileContext[*struct{}]) error {
			calls = append(calls, 2)
			return nil
		}),
	)

	rc := &reconciler.ReconcileContext[*struct{}]{Data: make(map[string]any)}
	err := chain.Execute(context.Background(), rc)

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, calls)
}
