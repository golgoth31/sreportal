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

package chain_test

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/portal/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"

	"github.com/stretchr/testify/require"
)

func newSchemeForSyncRemoteImageInventoryTest(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(scheme))
	return scheme
}

func TestSyncRemoteImageInventoryNoOpForLocalPortal(t *testing.T) {
	scheme := newSchemeForSyncRemoteImageInventoryTest(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	h := chain.NewSyncRemoteImageInventoryHandler(cli, scheme)

	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Main", Main: true},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Portal, chain.ChainData]{Resource: portal}
	require.NoError(t, h.Handle(context.Background(), rc))

	var list sreportalv1alpha1.ImageInventoryList
	require.NoError(t, cli.List(context.Background(), &list))
	require.Empty(t, list.Items)
}

func TestSyncRemoteImageInventoryCreatesShadowCRForRemotePortal(t *testing.T) {
	scheme := newSchemeForSyncRemoteImageInventoryTest(t)
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-a", Namespace: "default", UID: "uid-a"},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:  "Remote A",
			Remote: &sreportalv1alpha1.RemotePortalSpec{URL: "http://remote.example/", Portal: "main"},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()
	h := chain.NewSyncRemoteImageInventoryHandler(cli, scheme)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Portal, chain.ChainData]{Resource: portal}
	require.NoError(t, h.Handle(context.Background(), rc))

	got := &sreportalv1alpha1.ImageInventory{}
	require.NoError(t, cli.Get(context.Background(), types.NamespacedName{
		Name:      chain.RemoteImageInventoryName(portal.Name),
		Namespace: portal.Namespace,
	}, got))

	require.True(t, got.Spec.IsRemote)
	require.Equal(t, portal.Name, got.Spec.PortalRef)
	require.Equal(t, portal.Spec.Remote.URL, got.Spec.RemoteURL)
	require.NotEmpty(t, got.OwnerReferences)
	require.Equal(t, portal.UID, got.OwnerReferences[0].UID)
}

func TestSyncRemoteImageInventoryNoOpWhenFeatureDisabled(t *testing.T) {
	scheme := newSchemeForSyncRemoteImageInventoryTest(t)
	disabled := false
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-disabled", Namespace: "default"},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:    "Remote Disabled",
			Remote:   &sreportalv1alpha1.RemotePortalSpec{URL: "http://remote.example/", Portal: "main"},
			Features: &sreportalv1alpha1.PortalFeatures{ImageInventory: &disabled},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	h := chain.NewSyncRemoteImageInventoryHandler(cli, scheme)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Portal, chain.ChainData]{Resource: portal}
	require.NoError(t, h.Handle(context.Background(), rc))

	var list sreportalv1alpha1.ImageInventoryList
	require.NoError(t, cli.List(context.Background(), &list))
	require.Empty(t, list.Items)
}
