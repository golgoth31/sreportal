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

package flowobserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
)

// PrometheusObserver implements FlowObserver using Prometheus queries.
// It reads metric descriptors from the FlowObserver CRD spec.
type PrometheusObserver struct {
	api         promv1.API
	queryWindow time.Duration
	descriptors []sreportalv1alpha1.FlowMetricDescriptor
	mu          sync.Mutex
	active      *sreportalv1alpha1.FlowMetricDescriptor // detected mesh, nil until Available()
}

// PrometheusOption configures the PrometheusObserver.
type PrometheusOption func(*PrometheusObserver)

// WithPrometheusAPI overrides the default Prometheus API client (useful for testing).
func WithPrometheusAPI(api promv1.API) PrometheusOption {
	return func(o *PrometheusObserver) {
		o.api = api
	}
}

// NewPrometheusObserver creates a new Prometheus-based flow observer from CRD spec.
func NewPrometheusObserver(spec sreportalv1alpha1.FlowObserverSpec, opts ...PrometheusOption) (*PrometheusObserver, error) {
	queryWindow := 5 * time.Minute
	if spec.Prometheus.QueryWindow != "" {
		d, err := time.ParseDuration(spec.Prometheus.QueryWindow)
		if err == nil && d > 0 {
			queryWindow = d
		}
	}

	descriptors := spec.Metrics
	if len(descriptors) == 0 {
		descriptors = sreportalv1alpha1.DefaultMetricDescriptors()
	}

	o := &PrometheusObserver{
		queryWindow: queryWindow,
		descriptors: descriptors,
	}

	for _, opt := range opts {
		opt(o)
	}

	if o.api == nil {
		client, err := promapi.NewClient(promapi.Config{Address: spec.Prometheus.Address})
		if err != nil {
			return nil, fmt.Errorf("create prometheus client: %w", err)
		}

		o.api = promv1.NewAPI(client)
	}

	return o, nil
}

// Available probes Prometheus for known mesh metrics and returns true if any are found.
func (o *PrometheusObserver) Available(ctx context.Context) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.descriptors {
		desc := &o.descriptors[i]

		result, _, err := o.api.Query(ctx, desc.ProbeQuery, time.Now())
		if err != nil {
			continue
		}

		vec, ok := result.(model.Vector)
		if !ok || len(vec) == 0 {
			continue
		}

		o.active = desc

		return true, nil
	}

	return false, nil
}

// ActiveMesh returns the name of the detected mesh provider (for logging/status).
func (o *PrometheusObserver) ActiveMesh() string {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.active == nil {
		return ""
	}

	return o.active.Name
}

// Observed queries Prometheus for edges that have recent traffic.
func (o *PrometheusObserver) Observed(ctx context.Context, edges []domainnetpol.FlowEdge) (map[string]bool, error) {
	o.mu.Lock()
	active := o.active
	o.mu.Unlock()

	if active == nil {
		return nil, nil
	}

	// Build a lookup of edges by src/dst pair.
	type edgePair struct {
		srcNs, srcName string
		dstNs, dstName string
	}

	pairToKeys := make(map[edgePair][]string)

	for _, e := range edges {
		_, srcNs, srcName := ParseNodeID(e.From)
		_, dstNs, dstName := ParseNodeID(e.To)

		if srcNs == "" || dstNs == "" {
			continue
		}

		pair := edgePair{srcNs: srcNs, srcName: srcName, dstNs: dstNs, dstName: dstName}
		pairToKeys[pair] = append(pairToKeys[pair], domainnetpol.EdgeKey(e))
	}

	if len(pairToKeys) == 0 {
		return nil, nil
	}

	// Execute one aggregated query for all edges.
	window := formatPromDuration(o.queryWindow)
	query := fmt.Sprintf(active.ObservedQueryTemplate, window)

	result, _, err := o.api.Query(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("prometheus query failed: %w", err)
	}

	vec, ok := result.(model.Vector)
	if !ok {
		return nil, nil
	}

	// Match results to edges.
	observed := make(map[string]bool)

	for _, sample := range vec {
		srcNs := string(sample.Metric[model.LabelName(active.SourceNamespaceLabel)])
		srcName := string(sample.Metric[model.LabelName(active.SourceWorkloadLabel)])
		dstNs := string(sample.Metric[model.LabelName(active.DestinationNamespaceLabel)])
		dstName := string(sample.Metric[model.LabelName(active.DestinationWorkloadLabel)])

		pair := edgePair{srcNs: srcNs, srcName: srcName, dstNs: dstNs, dstName: dstName}

		keys, ok := pairToKeys[pair]
		if !ok {
			continue
		}

		for _, key := range keys {
			observed[key] = true
		}
	}

	return observed, nil
}

// formatPromDuration converts a Go duration to a Prometheus-compatible duration string.
func formatPromDuration(d time.Duration) string {
	if h := int(d.Hours()); h > 0 && d == time.Duration(h)*time.Hour {
		return fmt.Sprintf("%dh", h)
	}

	if m := int(d.Minutes()); m > 0 && d == time.Duration(m)*time.Minute {
		return fmt.Sprintf("%dm", m)
	}

	return fmt.Sprintf("%ds", int(d.Seconds()))
}
