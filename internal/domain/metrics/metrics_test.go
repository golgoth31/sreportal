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

package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainmetrics "github.com/golgoth31/sreportal/internal/domain/metrics"
)

const (
	nsSreportal      = "sreportal"
	subsystemDNS     = "dns"
	subsystemControl = "controller"
	metricFqdnsTotal = "fqdns_total"
	helpFqdnsTotal   = "Total FQDNs"
)

func TestGather_ReturnsOnlySreportalMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()

	// sreportal metric — should be returned
	sreportalGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: nsSreportal,
		Subsystem: subsystemDNS,
		Name:      metricFqdnsTotal,
		Help:      helpFqdnsTotal,
	})
	reg.MustRegister(sreportalGauge)
	sreportalGauge.Set(42)

	// non-sreportal metric — should be filtered out
	goGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_goroutines",
		Help: "Number of goroutines",
	})
	reg.MustRegister(goGauge)
	goGauge.Set(100)

	families, err := domainmetrics.Gather(reg, "", "")
	require.NoError(t, err)
	require.Len(t, families, 1)
	assert.Equal(t, "sreportal_dns_fqdns_total", families[0].Name)
	assert.Equal(t, "GAUGE", families[0].Type)
	require.Len(t, families[0].Metrics, 1)
	assert.Equal(t, 42.0, families[0].Metrics[0].Value)
}

func TestGather_SubsystemFilter(t *testing.T) {
	reg := prometheus.NewRegistry()

	dnsGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: nsSreportal,
		Subsystem: subsystemDNS,
		Name:      metricFqdnsTotal,
		Help:      helpFqdnsTotal,
	})
	controllerCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nsSreportal,
		Subsystem: subsystemControl,
		Name:      "reconcile_total",
		Help:      "Total reconciliations",
	})
	reg.MustRegister(dnsGauge, controllerCounter)
	dnsGauge.Set(10)
	controllerCounter.Add(5)

	families, err := domainmetrics.Gather(reg, "dns", "")
	require.NoError(t, err)
	require.Len(t, families, 1)
	assert.Equal(t, "sreportal_dns_fqdns_total", families[0].Name)
}

func TestGather_SearchFilter(t *testing.T) {
	reg := prometheus.NewRegistry()

	fqdnGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: nsSreportal,
		Subsystem: subsystemDNS,
		Name:      metricFqdnsTotal,
		Help:      helpFqdnsTotal,
	})
	groupGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: nsSreportal,
		Subsystem: subsystemDNS,
		Name:      "groups_total",
		Help:      "Total groups",
	})
	reg.MustRegister(fqdnGauge, groupGauge)

	families, err := domainmetrics.Gather(reg, "", "fqdn")
	require.NoError(t, err)
	require.Len(t, families, 1)
	assert.Equal(t, "sreportal_dns_fqdns_total", families[0].Name)
}

func TestGather_CounterValue(t *testing.T) {
	reg := prometheus.NewRegistry()

	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: nsSreportal,
		Subsystem: subsystemControl,
		Name:      "reconcile_total",
		Help:      "Total reconciliations",
	}, []string{"controller", "result"})
	reg.MustRegister(counter)
	counter.WithLabelValues("dns", "success").Add(10)
	counter.WithLabelValues("dns", "error").Add(2)

	families, err := domainmetrics.Gather(reg, "", "")
	require.NoError(t, err)
	require.Len(t, families, 1)
	assert.Equal(t, "COUNTER", families[0].Type)
	require.Len(t, families[0].Metrics, 2)

	// Find the success metric
	var found bool
	for _, m := range families[0].Metrics {
		if m.Labels["result"] == "success" {
			assert.Equal(t, 10.0, m.Value)
			assert.Equal(t, "dns", m.Labels["controller"])
			found = true
		}
	}
	assert.True(t, found, "expected to find metric with result=success")
}

func TestGather_HistogramValues(t *testing.T) {
	reg := prometheus.NewRegistry()

	hist := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsSreportal,
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request latency",
		Buckets:   []float64{0.01, 0.1, 1.0},
	})
	reg.MustRegister(hist)
	hist.Observe(0.05)
	hist.Observe(0.5)

	families, err := domainmetrics.Gather(reg, "", "")
	require.NoError(t, err)
	require.Len(t, families, 1)
	assert.Equal(t, "HISTOGRAM", families[0].Type)
	require.Len(t, families[0].Metrics, 1)

	m := families[0].Metrics[0]
	require.NotNil(t, m.Histogram)
	assert.Equal(t, uint64(2), m.Histogram.SampleCount)
	assert.Equal(t, 0.55, m.Histogram.SampleSum)
	require.Len(t, m.Histogram.Buckets, 4) // 3 explicit + +Inf
	assert.Equal(t, 0.01, m.Histogram.Buckets[0].UpperBound)
	assert.Equal(t, uint64(0), m.Histogram.Buckets[0].CumulativeCount)
	assert.Equal(t, 0.1, m.Histogram.Buckets[1].UpperBound)
	assert.Equal(t, uint64(1), m.Histogram.Buckets[1].CumulativeCount)
}

func TestGather_GaugeVecWithLabels(t *testing.T) {
	reg := prometheus.NewRegistry()

	gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: nsSreportal,
		Subsystem: subsystemDNS,
		Name:      metricFqdnsTotal,
		Help:      "Total FQDNs per portal",
	}, []string{"portal", "source"})
	reg.MustRegister(gaugeVec)
	gaugeVec.WithLabelValues("main", "external-dns").Set(5)
	gaugeVec.WithLabelValues("main", "manual").Set(3)

	families, err := domainmetrics.Gather(reg, "", "")
	require.NoError(t, err)
	require.Len(t, families, 1)
	require.Len(t, families[0].Metrics, 2)

	for _, m := range families[0].Metrics {
		assert.Contains(t, m.Labels, "portal")
		assert.Contains(t, m.Labels, "source")
	}
}

func TestGather_EmptyRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()

	families, err := domainmetrics.Gather(reg, "", "")
	require.NoError(t, err)
	assert.Empty(t, families)
}

func TestGather_CombinedFilters(t *testing.T) {
	reg := prometheus.NewRegistry()

	dnsGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: nsSreportal,
		Subsystem: subsystemDNS,
		Name:      metricFqdnsTotal,
		Help:      helpFqdnsTotal,
	})
	dnsGroups := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: nsSreportal,
		Subsystem: subsystemDNS,
		Name:      "groups_total",
		Help:      "Total groups",
	})
	httpCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nsSreportal,
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total HTTP requests",
	})
	reg.MustRegister(dnsGauge, dnsGroups, httpCounter)

	// subsystem=dns AND search=fqdn → only dns_fqdns_total
	families, err := domainmetrics.Gather(reg, "dns", "fqdn")
	require.NoError(t, err)
	require.Len(t, families, 1)
	assert.Equal(t, "sreportal_dns_fqdns_total", families[0].Name)
}
