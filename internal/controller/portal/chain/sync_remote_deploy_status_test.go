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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/portal/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"

	"github.com/stretchr/testify/require"
)

func TestSyncRemoteDeployStatusNoOpForLocalPortal(t *testing.T) {
	scheme := newSchemeForSyncRemoteImageInventoryTest(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	h := chain.NewSyncRemoteDeployStatusHandler(cli, scheme)

	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain, Namespace: nsDefault},
		Spec:       sreportalv1alpha1.PortalSpec{Title: tTitleMain, Main: true},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Portal, chain.ChainData]{Resource: portal}
	require.NoError(t, h.Handle(context.Background(), rc))

	var list sreportalv1alpha1.DeployStatusList
	require.NoError(t, cli.List(context.Background(), &list))
	require.Empty(t, list.Items)
}

func TestSyncRemoteDeployStatusCreatesShadowCRForRemotePortal(t *testing.T) {
	scheme := newSchemeForSyncRemoteImageInventoryTest(t)
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-a", Namespace: nsDefault, UID: "uid-a"},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:  "Remote A",
			Remote: &sreportalv1alpha1.RemotePortalSpec{URL: remoteURL, Portal: tPortalMain},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()
	h := chain.NewSyncRemoteDeployStatusHandler(cli, scheme)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Portal, chain.ChainData]{Resource: portal}
	require.NoError(t, h.Handle(context.Background(), rc))

	got := &sreportalv1alpha1.DeployStatus{}
	require.NoError(t, cli.Get(context.Background(), types.NamespacedName{
		Name:      chain.RemoteDeployStatusName(portal.Name),
		Namespace: portal.Namespace,
	}, got))

	require.True(t, got.Spec.IsRemote)
	require.Equal(t, portal.Name, got.Spec.PortalRef)
	require.Equal(t, chain.RemoteDeployStatusNamespace(portal.Name), got.Spec.Namespace)
	require.NotEmpty(t, got.OwnerReferences)
	require.Equal(t, portal.UID, got.OwnerReferences[0].UID)
}

func TestSyncRemoteDeployStatusUpdatesExistingShadowCR(t *testing.T) {
	scheme := newSchemeForSyncRemoteImageInventoryTest(t)
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-update", Namespace: nsDefault, UID: "uid-update"},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:  "Remote Update",
			Remote: &sreportalv1alpha1.RemotePortalSpec{URL: "http://old.example/", Portal: tPortalMain},
		},
	}
	stale := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chain.RemoteDeployStatusName(portal.Name),
			Namespace: portal.Namespace,
		},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: "wrong-portal",
			Namespace: "wrong-ns",
			IsRemote:  false,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, stale).Build()
	h := chain.NewSyncRemoteDeployStatusHandler(cli, scheme)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Portal, chain.ChainData]{Resource: portal}
	require.NoError(t, h.Handle(context.Background(), rc))

	got := &sreportalv1alpha1.DeployStatus{}
	require.NoError(t, cli.Get(context.Background(), types.NamespacedName{
		Name:      chain.RemoteDeployStatusName(portal.Name),
		Namespace: portal.Namespace,
	}, got))
	require.True(t, got.Spec.IsRemote)
	require.Equal(t, portal.Name, got.Spec.PortalRef)
	require.Equal(t, chain.RemoteDeployStatusNamespace(portal.Name), got.Spec.Namespace)
}

func TestSyncRemoteDeployStatusNoOpWhenFeatureDisabled(t *testing.T) {
	scheme := newSchemeForSyncRemoteImageInventoryTest(t)
	disabled := false
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-disabled", Namespace: nsDefault},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:    "Remote Disabled",
			Remote:   &sreportalv1alpha1.RemotePortalSpec{URL: remoteURL, Portal: tPortalMain},
			Features: &sreportalv1alpha1.PortalFeatures{DeployStatus: &disabled},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	h := chain.NewSyncRemoteDeployStatusHandler(cli, scheme)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Portal, chain.ChainData]{Resource: portal}
	require.NoError(t, h.Handle(context.Background(), rc))

	var list sreportalv1alpha1.DeployStatusList
	require.NoError(t, cli.List(context.Background(), &list))
	require.Empty(t, list.Items)
}

func TestCleanupDisabledFeaturesHandlerDeletesRemoteDeployStatusWhenFeatureDisabled(t *testing.T) {
	scheme := newSchemeForSyncRemoteImageInventoryTest(t)
	disabled := false
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-cleanup-ds", Namespace: nsDefault, UID: "uid-clean-ds"},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:    tTitleCleanup,
			Remote:   &sreportalv1alpha1.RemotePortalSpec{URL: remoteURL, Portal: tPortalMain},
			Features: &sreportalv1alpha1.PortalFeatures{DeployStatus: &disabled},
		},
	}
	shadow := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chain.RemoteDeployStatusName(portal.Name),
			Namespace: portal.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{APIVersion: tAPIVersion, Kind: tKindPortal, Name: portal.Name, UID: portal.UID},
			},
		},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: portal.Name,
			Namespace: chain.RemoteDeployStatusNamespace(portal.Name),
			IsRemote:  true,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, shadow).Build()
	h := chain.NewCleanupDisabledFeaturesHandler(cli)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Portal, chain.ChainData]{Resource: portal}
	require.NoError(t, h.Handle(context.Background(), rc))

	got := &sreportalv1alpha1.DeployStatus{}
	err := cli.Get(context.Background(), types.NamespacedName{
		Name:      chain.RemoteDeployStatusName(portal.Name),
		Namespace: portal.Namespace,
	}, got)
	require.True(t, apierrors.IsNotFound(err), "expected shadow DeployStatus to be deleted, got: %v", err)
}
