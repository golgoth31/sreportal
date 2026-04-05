package flowobserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
)

// meshDescriptor describes how a specific service mesh exposes flow metrics in Prometheus.
type meshDescriptor struct {
	Name          string // human-readable name
	ProbeQuery    string // PromQL to check existence
	LastSeenQuery string // PromQL returning max timestamp per src/dst pair
	SrcNamespace  string // label name for source namespace
	SrcWorkload   string // label name for source workload
	DstNamespace  string // label name for destination namespace
	DstWorkload   string // label name for destination workload
}

var meshDescriptors = []meshDescriptor{
	{
		Name:          "hubble",
		ProbeQuery:    `count(hubble_flows_processed_total)`,
		LastSeenQuery: `max by (source_workload, source_namespace, destination_workload, destination_namespace) (timestamp(hubble_flows_processed_total{verdict="FORWARDED"}))`,
		SrcNamespace:  "source_namespace",
		SrcWorkload:   "source_workload",
		DstNamespace:  "destination_namespace",
		DstWorkload:   "destination_workload",
	},
	{
		Name:          "istio",
		ProbeQuery:    `count(istio_requests_total)`,
		LastSeenQuery: `max by (source_workload, source_workload_namespace, destination_workload, destination_workload_namespace) (timestamp(istio_requests_total{reporter="destination"}))`,
		SrcNamespace:  "source_workload_namespace",
		SrcWorkload:   "source_workload",
		DstNamespace:  "destination_workload_namespace",
		DstWorkload:   "destination_workload",
	},
	{
		Name:          "linkerd",
		ProbeQuery:    `count(request_total{direction="outbound"})`,
		LastSeenQuery: `max by (namespace, deployment, dst_namespace, dst_deployment) (timestamp(request_total{direction="outbound"}))`,
		SrcNamespace:  "namespace",
		SrcWorkload:   "deployment",
		DstNamespace:  "dst_namespace",
		DstWorkload:   "dst_deployment",
	},
}

// PrometheusObserver implements FlowObserver using Prometheus queries.
type PrometheusObserver struct {
	api  promv1.API
	mu   sync.Mutex
	mesh *meshDescriptor // detected mesh, nil until Available() succeeds
}

// NewPrometheusObserver creates a new Prometheus-based flow observer.
func NewPrometheusObserver(address string) (*PrometheusObserver, error) {
	client, err := promapi.NewClient(promapi.Config{Address: address})
	if err != nil {
		return nil, fmt.Errorf("create prometheus client: %w", err)
	}

	return &PrometheusObserver{
		api: promv1.NewAPI(client),
	}, nil
}

// Available probes Prometheus for known mesh metrics and returns true if any are found.
func (o *PrometheusObserver) Available(ctx context.Context) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range meshDescriptors {
		desc := &meshDescriptors[i]

		result, _, err := o.api.Query(ctx, desc.ProbeQuery, time.Now())
		if err != nil {
			continue
		}

		vec, ok := result.(model.Vector)
		if !ok || len(vec) == 0 {
			continue
		}

		o.mesh = desc

		return true, nil
	}

	return false, nil
}

// MeshName returns the name of the detected mesh provider (for logging).
func (o *PrometheusObserver) MeshName() string {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.mesh == nil {
		return ""
	}

	return o.mesh.Name
}

// LastSeen queries Prometheus for the most recent traffic timestamp per edge.
func (o *PrometheusObserver) LastSeen(ctx context.Context, edges []domainnetpol.FlowEdge) (map[string]time.Time, error) {
	o.mu.Lock()
	mesh := o.mesh
	o.mu.Unlock()

	if mesh == nil {
		return nil, nil
	}

	// Build a lookup of edges by from|to pair (ignoring edge type — Prometheus doesn't know edge types).
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
	result, _, err := o.api.Query(ctx, mesh.LastSeenQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("prometheus query failed: %w", err)
	}

	vec, ok := result.(model.Vector)
	if !ok {
		return nil, nil
	}

	// Match results to edges.
	lastSeen := make(map[string]time.Time)

	for _, sample := range vec {
		srcNs := string(sample.Metric[model.LabelName(mesh.SrcNamespace)])
		srcName := string(sample.Metric[model.LabelName(mesh.SrcWorkload)])
		dstNs := string(sample.Metric[model.LabelName(mesh.DstNamespace)])
		dstName := string(sample.Metric[model.LabelName(mesh.DstWorkload)])

		pair := edgePair{srcNs: srcNs, srcName: srcName, dstNs: dstNs, dstName: dstName}

		keys, ok := pairToKeys[pair]
		if !ok {
			continue
		}

		// The sample value is the Unix timestamp (from PromQL timestamp() function).
		ts := time.Unix(int64(sample.Value), 0)

		for _, key := range keys {
			if existing, ok := lastSeen[key]; !ok || ts.After(existing) {
				lastSeen[key] = ts
			}
		}
	}

	return lastSeen, nil
}
