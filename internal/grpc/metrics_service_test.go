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

	"connectrpc.com/connect"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	metricsv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
)

func TestListMetrics_ReturnsOnlySreportalMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()

	sreportalGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "sreportal",
		Subsystem: "dns",
		Name:      "fqdns_total",
		Help:      "Total FQDNs",
	})
	goGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_goroutines",
		Help: "Number of goroutines",
	})
	reg.MustRegister(sreportalGauge, goGauge)
	sreportalGauge.Set(42)
	goGauge.Set(100)

	svc := svcgrpc.NewMetricsService(reg)
	resp, err := svc.ListMetrics(context.Background(), connect.NewRequest(&metricsv1.ListMetricsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Families, 1)
	assert.Equal(t, "sreportal_dns_fqdns_total", resp.Msg.Families[0].Name)
	assert.Equal(t, "GAUGE", resp.Msg.Families[0].Type)
	require.Len(t, resp.Msg.Families[0].Metrics, 1)
	assert.Equal(t, 42.0, resp.Msg.Families[0].Metrics[0].Value)
}

func TestListMetrics_SubsystemFilter(t *testing.T) {
	reg := prometheus.NewRegistry()

	dnsGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "sreportal",
		Subsystem: "dns",
		Name:      "fqdns_total",
		Help:      "Total FQDNs",
	})
	controllerCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "sreportal",
		Subsystem: "controller",
		Name:      "reconcile_total",
		Help:      "Total reconciliations",
	})
	reg.MustRegister(dnsGauge, controllerCounter)

	svc := svcgrpc.NewMetricsService(reg)
	resp, err := svc.ListMetrics(context.Background(), connect.NewRequest(&metricsv1.ListMetricsRequest{
		Subsystem: "controller",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Families, 1)
	assert.Equal(t, "sreportal_controller_reconcile_total", resp.Msg.Families[0].Name)
}

func TestListMetrics_SearchFilter(t *testing.T) {
	reg := prometheus.NewRegistry()

	fqdnGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "sreportal",
		Subsystem: "dns",
		Name:      "fqdns_total",
		Help:      "Total FQDNs",
	})
	groupGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "sreportal",
		Subsystem: "dns",
		Name:      "groups_total",
		Help:      "Total groups",
	})
	reg.MustRegister(fqdnGauge, groupGauge)

	svc := svcgrpc.NewMetricsService(reg)
	resp, err := svc.ListMetrics(context.Background(), connect.NewRequest(&metricsv1.ListMetricsRequest{
		Search: "fqdn",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Families, 1)
	assert.Equal(t, "sreportal_dns_fqdns_total", resp.Msg.Families[0].Name)
}

func TestListMetrics_HistogramValues(t *testing.T) {
	reg := prometheus.NewRegistry()

	hist := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "sreportal",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request latency",
		Buckets:   []float64{0.01, 0.1, 1.0},
	})
	reg.MustRegister(hist)
	hist.Observe(0.05)
	hist.Observe(0.5)

	svc := svcgrpc.NewMetricsService(reg)
	resp, err := svc.ListMetrics(context.Background(), connect.NewRequest(&metricsv1.ListMetricsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Families, 1)
	assert.Equal(t, "HISTOGRAM", resp.Msg.Families[0].Type)
	require.Len(t, resp.Msg.Families[0].Metrics, 1)

	m := resp.Msg.Families[0].Metrics[0]
	require.NotNil(t, m.Histogram)
	assert.Equal(t, uint64(2), m.Histogram.SampleCount)
	assert.Equal(t, 0.55, m.Histogram.SampleSum)
	require.GreaterOrEqual(t, len(m.Histogram.Buckets), 3)
}

func TestListMetrics_EmptyRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()

	svc := svcgrpc.NewMetricsService(reg)
	resp, err := svc.ListMetrics(context.Background(), connect.NewRequest(&metricsv1.ListMetricsRequest{}))
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Families)
}
