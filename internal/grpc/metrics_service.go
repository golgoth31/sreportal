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

package grpc

import (
	"context"

	"connectrpc.com/connect"
	"github.com/prometheus/client_golang/prometheus"

	domainmetrics "github.com/golgoth31/sreportal/internal/domain/metrics"
	metricsv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// MetricsService implements the MetricsServiceHandler interface.
type MetricsService struct {
	sreportalv1connect.UnimplementedMetricsServiceHandler
	gatherer prometheus.Gatherer
}

// NewMetricsService creates a new MetricsService.
func NewMetricsService(g prometheus.Gatherer) *MetricsService {
	return &MetricsService{gatherer: g}
}

// ListMetrics returns current values of sreportal custom metrics.
func (s *MetricsService) ListMetrics(
	_ context.Context,
	req *connect.Request[metricsv1.ListMetricsRequest],
) (*connect.Response[metricsv1.ListMetricsResponse], error) {
	families, err := domainmetrics.Gather(s.gatherer, req.Msg.Subsystem, req.Msg.Search)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoFamilies := make([]*metricsv1.MetricFamily, 0, len(families))
	for _, f := range families {
		protoFamilies = append(protoFamilies, toProtoMetricFamily(f))
	}

	return connect.NewResponse(&metricsv1.ListMetricsResponse{
		Families: protoFamilies,
	}), nil
}

func toProtoMetricFamily(f domainmetrics.MetricFamily) *metricsv1.MetricFamily {
	protoMetrics := make([]*metricsv1.Metric, 0, len(f.Metrics))
	for _, m := range f.Metrics {
		protoMetrics = append(protoMetrics, toProtoMetric(m))
	}
	return &metricsv1.MetricFamily{
		Name:    f.Name,
		Help:    f.Help,
		Type:    f.Type,
		Metrics: protoMetrics,
	}
}

func toProtoMetric(m domainmetrics.Metric) *metricsv1.Metric {
	pm := &metricsv1.Metric{
		Labels: m.Labels,
		Value:  m.Value,
	}
	if m.Histogram != nil {
		buckets := make([]*metricsv1.HistogramBucket, 0, len(m.Histogram.Buckets))
		for _, b := range m.Histogram.Buckets {
			buckets = append(buckets, &metricsv1.HistogramBucket{
				CumulativeCount: b.CumulativeCount,
				UpperBound:      b.UpperBound,
			})
		}
		pm.Histogram = &metricsv1.HistogramValue{
			SampleCount: m.Histogram.SampleCount,
			SampleSum:   m.Histogram.SampleSum,
			Buckets:     buckets,
		}
	}
	return pm
}
