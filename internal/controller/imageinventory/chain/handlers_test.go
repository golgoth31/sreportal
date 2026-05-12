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

package chain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const tForeignCondition = "ForeignCondition"

// TestUpdateStatusHandlerPreservesForeignCondition verifies that bumping
// ObservedGeneration uses a Patch (MergeFrom) rather than a full Status().Update
// so that concurrent writes to other status fields (e.g. a condition set by
// another writer between our Reconcile's Get and this handler) are not
// clobbered by our stale snapshot.
func TestUpdateStatusHandlerPreservesForeignCondition(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(scheme))

	const (
		name = "inv-update-status"
		ns   = tNsDefault
	)

	// Inventory at generation=2 with stale observedGeneration=1.
	stored := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  ns,
			Generation: 2,
		},
		Spec: sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalMain},
		Status: sreportalv1alpha1.ImageInventoryStatus{
			ObservedGeneration: 1,
		},
	}

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&sreportalv1alpha1.ImageInventory{}).
		WithObjects(stored).
		Build()

	// Simulate a concurrent writer: append a foreign condition to the live
	// object via the fake client. This mimics state-of-world AFTER our
	// Reconcile fetched its snapshot but BEFORE UpdateStatusHandler runs.
	live := &sreportalv1alpha1.ImageInventory{}
	require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: name, Namespace: ns}, live))
	live.Status.Conditions = append(live.Status.Conditions, metav1.Condition{
		Type:               tForeignCondition,
		Status:             metav1.ConditionTrue,
		Reason:             "SetByOther",
		Message:            "foreign writer",
		LastTransitionTime: metav1.Now(),
	})
	require.NoError(t, cli.Status().Update(context.Background(), live))

	// The inventory we hand to the handler is our STALE snapshot: it does NOT
	// see the ForeignCondition.
	staleSnapshot := stored.DeepCopy()
	require.Empty(t, staleSnapshot.Status.Conditions)

	h := NewUpdateStatusHandler(cli)
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: staleSnapshot}
	require.NoError(t, h.Handle(context.Background(), rc))

	// Re-fetch and assert.
	got := &sreportalv1alpha1.ImageInventory{}
	require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: name, Namespace: ns}, got))

	require.Equal(t, int64(2), got.Status.ObservedGeneration, "observedGeneration must be bumped to current generation")

	var hasForeign bool
	for _, c := range got.Status.Conditions {
		if c.Type == tForeignCondition {
			hasForeign = true
			break
		}
	}
	require.True(t, hasForeign, "foreign condition must be preserved — a full Status().Update with a stale snapshot would clobber it")
}

