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

package metrics

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

const namespacePrefix = "sreportal_"

// MetricFamily represents a group of metrics sharing a name, help text, and type.
type MetricFamily struct {
	Name    string   `json:"name"`
	Help    string   `json:"help"`
	Type    string   `json:"type"`
	Metrics []Metric `json:"metrics"`
}

// Metric represents a single metric with its labels and value.
type Metric struct {
	Labels    map[string]string `json:"labels,omitempty"`
	Value     float64           `json:"value"`
	Histogram *HistogramValue   `json:"histogram,omitempty"`
}

// HistogramValue holds histogram-specific data.
type HistogramValue struct {
	SampleCount uint64            `json:"sample_count"`
	SampleSum   float64           `json:"sample_sum"`
	Buckets     []HistogramBucket `json:"buckets"`
}

// HistogramBucket represents a single histogram bucket.
type HistogramBucket struct {
	CumulativeCount uint64  `json:"cumulative_count"`
	UpperBound      float64 `json:"upper_bound"`
}

// Gather collects metrics from the given gatherer, filtering to sreportal_* metrics only.
// Optional subsystem filters to sreportal_{subsystem}_* prefix.
// Optional search filters by substring match on the metric name.
func Gather(gatherer prometheus.Gatherer, subsystem, search string) ([]MetricFamily, error) {
	gathered, err := gatherer.Gather()
	if err != nil {
		return nil, fmt.Errorf("gather metrics: %w", err)
	}

	subsystemPrefix := ""
	if subsystem != "" {
		subsystemPrefix = namespacePrefix + subsystem + "_"
	}
	searchLower := strings.ToLower(search)

	var families []MetricFamily
	for _, mf := range gathered {
		name := mf.GetName()

		if !strings.HasPrefix(name, namespacePrefix) {
			continue
		}
		if subsystemPrefix != "" && !strings.HasPrefix(name, subsystemPrefix) {
			continue
		}
		if searchLower != "" && !strings.Contains(strings.ToLower(name), searchLower) {
			continue
		}

		families = append(families, convertFamily(mf))
	}

	sort.Slice(families, func(i, j int) bool {
		return families[i].Name < families[j].Name
	})

	return families, nil
}

func convertFamily(mf *dto.MetricFamily) MetricFamily {
	f := MetricFamily{
		Name: mf.GetName(),
		Help: mf.GetHelp(),
		Type: metricTypeName(mf.GetType()),
	}

	for _, m := range mf.GetMetric() {
		f.Metrics = append(f.Metrics, convertMetric(mf.GetType(), m))
	}

	return f
}

func convertMetric(typ dto.MetricType, m *dto.Metric) Metric {
	metric := Metric{
		Labels: extractLabels(m),
	}

	switch typ {
	case dto.MetricType_COUNTER:
		metric.Value = m.GetCounter().GetValue()
	case dto.MetricType_GAUGE:
		metric.Value = m.GetGauge().GetValue()
	case dto.MetricType_HISTOGRAM:
		h := m.GetHistogram()
		hv := &HistogramValue{
			SampleCount: h.GetSampleCount(),
			SampleSum:   h.GetSampleSum(),
		}
		for _, b := range h.GetBucket() {
			hv.Buckets = append(hv.Buckets, HistogramBucket{
				CumulativeCount: b.GetCumulativeCount(),
				UpperBound:      b.GetUpperBound(),
			})
		}
		// Add the +Inf bucket (always present in Prometheus histograms).
		if len(hv.Buckets) == 0 || hv.Buckets[len(hv.Buckets)-1].UpperBound != math.Inf(1) {
			hv.Buckets = append(hv.Buckets, HistogramBucket{
				CumulativeCount: h.GetSampleCount(),
				UpperBound:      math.Inf(1),
			})
		}
		metric.Histogram = hv
	}

	return metric
}

func extractLabels(m *dto.Metric) map[string]string {
	pairs := m.GetLabel()
	if len(pairs) == 0 {
		return nil
	}
	labels := make(map[string]string, len(pairs))
	for _, lp := range pairs {
		labels[lp.GetName()] = lp.GetValue()
	}
	return labels
}

func metricTypeName(t dto.MetricType) string {
	switch t {
	case dto.MetricType_COUNTER:
		return "COUNTER"
	case dto.MetricType_GAUGE:
		return "GAUGE"
	case dto.MetricType_HISTOGRAM:
		return "HISTOGRAM"
	case dto.MetricType_SUMMARY:
		return "SUMMARY"
	default:
		return "UNTYPED"
	}
}
