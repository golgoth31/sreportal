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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	alertmanagerv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
)

func TestListAlerts_ReturnsAlertmanagerResources(t *testing.T) {
	scheme := newScheme(t)
	now := metav1.Now()
	startsAt := metav1.NewTime(time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC))

	am := &sreportalv1alpha1.Alertmanager{
		ObjectMeta: metav1.ObjectMeta{Name: "am-prod", Namespace: "monitoring"},
		Spec: sreportalv1alpha1.AlertmanagerSpec{
			PortalRef: "main",
			URL: sreportalv1alpha1.AlertmanagerURL{
				Local:  "http://alertmanager:9093",
				Remote: "https://alertmanager.example.com",
			},
		},
		Status: sreportalv1alpha1.AlertmanagerStatus{
			ActiveAlerts: []sreportalv1alpha1.AlertStatus{
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
			LastReconcileTime: &now,
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "ReconcileSucceeded", LastTransitionTime: now},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(am).Build()
	svc := svcgrpc.NewAlertmanagerService(c)

	resp, err := svc.ListAlerts(context.Background(), connect.NewRequest(&alertmanagerv1.ListAlertsRequest{}))
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
	scheme := newScheme(t)

	am1 := &sreportalv1alpha1.Alertmanager{
		ObjectMeta: metav1.ObjectMeta{Name: "am-main", Namespace: "default"},
		Spec: sreportalv1alpha1.AlertmanagerSpec{
			PortalRef: "main",
			URL:       sreportalv1alpha1.AlertmanagerURL{Local: "http://am1:9093"},
		},
	}
	am2 := &sreportalv1alpha1.Alertmanager{
		ObjectMeta: metav1.ObjectMeta{Name: "am-other", Namespace: "default"},
		Spec: sreportalv1alpha1.AlertmanagerSpec{
			PortalRef: "other",
			URL:       sreportalv1alpha1.AlertmanagerURL{Local: "http://am2:9093"},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(am1, am2).Build()
	svc := svcgrpc.NewAlertmanagerService(c)

	resp, err := svc.ListAlerts(context.Background(), connect.NewRequest(&alertmanagerv1.ListAlertsRequest{Portal: "main"}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Alertmanagers, 1)
	assert.Equal(t, "am-main", resp.Msg.Alertmanagers[0].Name)
}

func TestListAlerts_FiltersBySearch(t *testing.T) {
	scheme := newScheme(t)
	startsAt := metav1.NewTime(time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC))

	am := &sreportalv1alpha1.Alertmanager{
		ObjectMeta: metav1.ObjectMeta{Name: "am-test", Namespace: "default"},
		Spec: sreportalv1alpha1.AlertmanagerSpec{
			PortalRef: "main",
			URL:       sreportalv1alpha1.AlertmanagerURL{Local: "http://am:9093"},
		},
		Status: sreportalv1alpha1.AlertmanagerStatus{
			ActiveAlerts: []sreportalv1alpha1.AlertStatus{
				{Fingerprint: "aaa", Labels: map[string]string{"alertname": "HighCPU"}, State: "active", StartsAt: startsAt, UpdatedAt: startsAt},
				{Fingerprint: "bbb", Labels: map[string]string{"alertname": "DiskFull"}, State: "active", StartsAt: startsAt, UpdatedAt: startsAt},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(am).Build()
	svc := svcgrpc.NewAlertmanagerService(c)

	resp, err := svc.ListAlerts(context.Background(), connect.NewRequest(&alertmanagerv1.ListAlertsRequest{Search: "cpu"}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Alertmanagers, 1)
	require.Len(t, resp.Msg.Alertmanagers[0].Alerts, 1)
	assert.Equal(t, "aaa", resp.Msg.Alertmanagers[0].Alerts[0].Fingerprint)
}

func TestListAlerts_FiltersByState(t *testing.T) {
	scheme := newScheme(t)
	startsAt := metav1.NewTime(time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC))

	am := &sreportalv1alpha1.Alertmanager{
		ObjectMeta: metav1.ObjectMeta{Name: "am-test", Namespace: "default"},
		Spec: sreportalv1alpha1.AlertmanagerSpec{
			PortalRef: "main",
			URL:       sreportalv1alpha1.AlertmanagerURL{Local: "http://am:9093"},
		},
		Status: sreportalv1alpha1.AlertmanagerStatus{
			ActiveAlerts: []sreportalv1alpha1.AlertStatus{
				{Fingerprint: "aaa", Labels: map[string]string{"alertname": "A"}, State: "active", StartsAt: startsAt, UpdatedAt: startsAt},
				{Fingerprint: "bbb", Labels: map[string]string{"alertname": "B"}, State: "suppressed", StartsAt: startsAt, UpdatedAt: startsAt},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(am).Build()
	svc := svcgrpc.NewAlertmanagerService(c)

	resp, err := svc.ListAlerts(context.Background(), connect.NewRequest(&alertmanagerv1.ListAlertsRequest{State: "suppressed"}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Alertmanagers, 1)
	require.Len(t, resp.Msg.Alertmanagers[0].Alerts, 1)
	assert.Equal(t, "bbb", resp.Msg.Alertmanagers[0].Alerts[0].Fingerprint)
}

func TestListAlerts_UsesSpecRemoteURL(t *testing.T) {
	scheme := newScheme(t)
	startsAt := metav1.NewTime(time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC))

	am := &sreportalv1alpha1.Alertmanager{
		ObjectMeta: metav1.ObjectMeta{Name: "am-remote", Namespace: "default"},
		Spec: sreportalv1alpha1.AlertmanagerSpec{
			PortalRef: "remote-portal",
			URL:       sreportalv1alpha1.AlertmanagerURL{Local: "http://portal:8080", Remote: "https://real-alertmanager.example.com"},
			IsRemote:  true,
		},
		Status: sreportalv1alpha1.AlertmanagerStatus{
			ActiveAlerts: []sreportalv1alpha1.AlertStatus{
				{Fingerprint: "aaa", Labels: map[string]string{"alertname": "HighCPU"}, State: "active", StartsAt: startsAt, UpdatedAt: startsAt},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(am).Build()
	svc := svcgrpc.NewAlertmanagerService(c)

	resp, err := svc.ListAlerts(context.Background(), connect.NewRequest(&alertmanagerv1.ListAlertsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Alertmanagers, 1)
	assert.Equal(t, "https://real-alertmanager.example.com", resp.Msg.Alertmanagers[0].RemoteUrl)
}

func TestListAlerts_WhenNoAlertmanagers_ReturnsEmpty(t *testing.T) {
	scheme := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	svc := svcgrpc.NewAlertmanagerService(c)

	resp, err := svc.ListAlerts(context.Background(), connect.NewRequest(&alertmanagerv1.ListAlertsRequest{}))
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Alertmanagers)
}
