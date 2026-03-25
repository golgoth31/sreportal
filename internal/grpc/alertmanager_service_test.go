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

package grpc_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanagerreadmodel"
	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	alertmanagerv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	amstore "github.com/golgoth31/sreportal/internal/readstore/alertmanager"
)

func TestListAlerts_ReturnsAlertmanagerResources(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	startsAt := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)

	store := amstore.NewAlertmanagerStore()
	_ = store.Replace(ctx, "monitoring/am-prod", domainalertmanager.AlertmanagerView{
		Name:              "am-prod",
		Namespace:         "monitoring",
		PortalRef:         "main",
		LocalURL:          "http://alertmanager:9093",
		RemoteURL:         "https://alertmanager.example.com",
		Ready:             true,
		LastReconcileTime: &now,
		Alerts: []domainalertmanager.AlertView{
			{
				Fingerprint: "aaa",
				Labels:      map[string]string{"alertname": "HighCPU", "severity": "critical"},
				Annotations: map[string]string{"summary": "CPU usage above 90%"},
				State:       "active",
				StartsAt:    startsAt,
				UpdatedAt:   startsAt,
			},
			{
				Fingerprint: "bbb",
				Labels:      map[string]string{"alertname": "DiskFull", "severity": "warning"},
				State:       "active",
				StartsAt:    startsAt,
				UpdatedAt:   startsAt,
			},
		},
	})

	svc := svcgrpc.NewAlertmanagerService(store)

	resp, err := svc.ListAlerts(ctx, connect.NewRequest(&alertmanagerv1.ListAlertsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Alertmanagers, 1)

	res := resp.Msg.Alertmanagers[0]
	assert.Equal(t, "am-prod", res.Name)
	assert.Equal(t, "monitoring", res.Namespace)
	assert.Equal(t, "main", res.PortalRef)
	assert.Equal(t, "http://alertmanager:9093", res.LocalUrl)
	assert.Equal(t, "https://alertmanager.example.com", res.RemoteUrl)
	assert.True(t, res.Ready)
	assert.NotNil(t, res.LastReconcileTime)
	require.Len(t, res.Alerts, 2)
	assert.Equal(t, "aaa", res.Alerts[0].Fingerprint)
	assert.Equal(t, "HighCPU", res.Alerts[0].Labels["alertname"])
	assert.Equal(t, "CPU usage above 90%", res.Alerts[0].Annotations["summary"])
}

func TestListAlerts_FiltersByPortal(t *testing.T) {
	ctx := context.Background()
	store := amstore.NewAlertmanagerStore()
	_ = store.Replace(ctx, "default/am-main", domainalertmanager.AlertmanagerView{
		Name: "am-main", Namespace: "default", PortalRef: "main",
		LocalURL: "http://am1:9093",
	})
	_ = store.Replace(ctx, "default/am-other", domainalertmanager.AlertmanagerView{
		Name: "am-other", Namespace: "default", PortalRef: "other",
		LocalURL: "http://am2:9093",
	})

	svc := svcgrpc.NewAlertmanagerService(store)

	resp, err := svc.ListAlerts(ctx, connect.NewRequest(&alertmanagerv1.ListAlertsRequest{Portal: "main"}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Alertmanagers, 1)
	assert.Equal(t, "am-main", resp.Msg.Alertmanagers[0].Name)
}

func TestListAlerts_FiltersBySearch(t *testing.T) {
	ctx := context.Background()
	startsAt := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)

	store := amstore.NewAlertmanagerStore()
	_ = store.Replace(ctx, "default/am-test", domainalertmanager.AlertmanagerView{
		Name: "am-test", Namespace: "default", PortalRef: "main",
		LocalURL: "http://am:9093",
		Alerts: []domainalertmanager.AlertView{
			{Fingerprint: "aaa", Labels: map[string]string{"alertname": "HighCPU"}, State: "active", StartsAt: startsAt, UpdatedAt: startsAt},
			{Fingerprint: "bbb", Labels: map[string]string{"alertname": "DiskFull"}, State: "active", StartsAt: startsAt, UpdatedAt: startsAt},
		},
	})

	svc := svcgrpc.NewAlertmanagerService(store)

	resp, err := svc.ListAlerts(ctx, connect.NewRequest(&alertmanagerv1.ListAlertsRequest{Search: "cpu"}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Alertmanagers, 1)
	require.Len(t, resp.Msg.Alertmanagers[0].Alerts, 1)
	assert.Equal(t, "aaa", resp.Msg.Alertmanagers[0].Alerts[0].Fingerprint)
}

func TestListAlerts_FiltersByState(t *testing.T) {
	ctx := context.Background()
	startsAt := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)

	store := amstore.NewAlertmanagerStore()
	_ = store.Replace(ctx, "default/am-test", domainalertmanager.AlertmanagerView{
		Name: "am-test", Namespace: "default", PortalRef: "main",
		LocalURL: "http://am:9093",
		Alerts: []domainalertmanager.AlertView{
			{Fingerprint: "aaa", Labels: map[string]string{"alertname": "A"}, State: "active", StartsAt: startsAt, UpdatedAt: startsAt},
			{Fingerprint: "bbb", Labels: map[string]string{"alertname": "B"}, State: "suppressed", StartsAt: startsAt, UpdatedAt: startsAt},
		},
	})

	svc := svcgrpc.NewAlertmanagerService(store)

	resp, err := svc.ListAlerts(ctx, connect.NewRequest(&alertmanagerv1.ListAlertsRequest{State: "suppressed"}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Alertmanagers, 1)
	require.Len(t, resp.Msg.Alertmanagers[0].Alerts, 1)
	assert.Equal(t, "bbb", resp.Msg.Alertmanagers[0].Alerts[0].Fingerprint)
}

func TestListAlerts_UsesSpecRemoteURL(t *testing.T) {
	ctx := context.Background()
	startsAt := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)

	store := amstore.NewAlertmanagerStore()
	_ = store.Replace(ctx, "default/am-remote", domainalertmanager.AlertmanagerView{
		Name: "am-remote", Namespace: "default", PortalRef: "remote-portal",
		LocalURL:  "http://portal:8080",
		RemoteURL: "https://real-alertmanager.example.com",
		Alerts: []domainalertmanager.AlertView{
			{Fingerprint: "aaa", Labels: map[string]string{"alertname": "HighCPU"}, State: "active", StartsAt: startsAt, UpdatedAt: startsAt},
		},
	})

	svc := svcgrpc.NewAlertmanagerService(store)

	resp, err := svc.ListAlerts(ctx, connect.NewRequest(&alertmanagerv1.ListAlertsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Alertmanagers, 1)
	assert.Equal(t, "https://real-alertmanager.example.com", resp.Msg.Alertmanagers[0].RemoteUrl)
}

func TestListAlerts_WhenNoAlertmanagers_ReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	store := amstore.NewAlertmanagerStore()
	svc := svcgrpc.NewAlertmanagerService(store)

	resp, err := svc.ListAlerts(ctx, connect.NewRequest(&alertmanagerv1.ListAlertsRequest{}))
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Alertmanagers)
}
