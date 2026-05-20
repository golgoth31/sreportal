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

type testData struct {
	Value string
}

func TestChain_Execute_StopsOnError(t *testing.T) {
	var secondCalled bool
	chain := reconciler.NewChain[*struct{}, testData](
		"",
		reconciler.HandlerFunc[*struct{}, testData](func(_ context.Context, _ *reconciler.ReconcileContext[*struct{}, testData]) error {
			return errors.New("first handler failed")
		}),
		reconciler.HandlerFunc[*struct{}, testData](func(_ context.Context, _ *reconciler.ReconcileContext[*struct{}, testData]) error {
			secondCalled = true
			return nil
		}),
	)

	rc := &reconciler.ReconcileContext[*struct{}, testData]{}
	err := chain.Execute(context.Background(), rc)

	require.Error(t, err)
	assert.False(t, secondCalled, "second handler must not run after an error")
}

func TestChain_Execute_StopsOnRequeueAfter(t *testing.T) {
	var secondCalled bool
	chain := reconciler.NewChain[*struct{}, testData](
		"",
		reconciler.HandlerFunc[*struct{}, testData](func(_ context.Context, rc *reconciler.ReconcileContext[*struct{}, testData]) error {
			rc.Result.RequeueAfter = 5 * time.Second
			return nil
		}),
		reconciler.HandlerFunc[*struct{}, testData](func(_ context.Context, _ *reconciler.ReconcileContext[*struct{}, testData]) error {
			secondCalled = true
			return nil
		}),
	)

	rc := &reconciler.ReconcileContext[*struct{}, testData]{}
	err := chain.Execute(context.Background(), rc)

	require.NoError(t, err)
	assert.False(t, secondCalled, "second handler must not run after RequeueAfter is set")
}

func TestChain_Execute_RunsAllHandlers_WhenNoShortCircuit(t *testing.T) {
	var calls []int
	chain := reconciler.NewChain[*struct{}, testData](
		"",
		reconciler.HandlerFunc[*struct{}, testData](func(_ context.Context, _ *reconciler.ReconcileContext[*struct{}, testData]) error {
			calls = append(calls, 1)
			return nil
		}),
		reconciler.HandlerFunc[*struct{}, testData](func(_ context.Context, _ *reconciler.ReconcileContext[*struct{}, testData]) error {
			calls = append(calls, 2)
			return nil
		}),
	)

	rc := &reconciler.ReconcileContext[*struct{}, testData]{}
	err := chain.Execute(context.Background(), rc)

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, calls)
}

func TestChain_Execute_SwallowsCtxErr_WhenParentDone(t *testing.T) {
	// Parent ctx is canceled — handler returns context.Canceled. This is a
	// shutdown / re-queue race, not a real failure, so Execute must swallow.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	chain := reconciler.NewChain[*struct{}, testData](
		"",
		reconciler.HandlerFunc[*struct{}, testData](func(_ context.Context, _ *reconciler.ReconcileContext[*struct{}, testData]) error {
			return context.Canceled
		}),
	)
	rc := &reconciler.ReconcileContext[*struct{}, testData]{}
	require.NoError(t, chain.Execute(ctx, rc))
}

func TestChain_Execute_PropagatesCtxErr_WhenParentAlive(t *testing.T) {
	// Parent ctx is alive — handler returned context.DeadlineExceeded because
	// of its own per-call timeout. The chain must propagate so controller-
	// runtime sees the failure and retries.
	chain := reconciler.NewChain[*struct{}, testData](
		"",
		reconciler.HandlerFunc[*struct{}, testData](func(_ context.Context, _ *reconciler.ReconcileContext[*struct{}, testData]) error {
			return context.DeadlineExceeded
		}),
	)
	rc := &reconciler.ReconcileContext[*struct{}, testData]{}
	err := chain.Execute(context.Background(), rc)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestChain_Execute_SharesTypedData(t *testing.T) {
	chain := reconciler.NewChain[*struct{}, testData](
		"",
		reconciler.HandlerFunc[*struct{}, testData](func(_ context.Context, rc *reconciler.ReconcileContext[*struct{}, testData]) error {
			rc.Data.Value = "hello"
			return nil
		}),
		reconciler.HandlerFunc[*struct{}, testData](func(_ context.Context, rc *reconciler.ReconcileContext[*struct{}, testData]) error {
			rc.Data.Value += " world"
			return nil
		}),
	)

	rc := &reconciler.ReconcileContext[*struct{}, testData]{}
	err := chain.Execute(context.Background(), rc)

	require.NoError(t, err)
	assert.Equal(t, "hello world", rc.Data.Value)
}
