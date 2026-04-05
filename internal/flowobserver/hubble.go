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
	"io"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	flowpb "github.com/golgoth31/sreportal/internal/flowobserver/hubblegen/flow"
	observerpb "github.com/golgoth31/sreportal/internal/flowobserver/hubblegen/observer"
)

const (
	defaultHubbleAddress = "hubble-relay.kube-system.svc.cluster.local:4245"
	hubbleDialTimeout    = 2 * time.Second
	defaultFlowWindow    = 5 * time.Minute
)

// HubbleObserver implements FlowObserver using the Hubble gRPC Observer API.
type HubbleObserver struct {
	address    string
	flowWindow time.Duration
	dialOpts   []grpc.DialOption
	mu         sync.Mutex
	conn       *grpc.ClientConn
}

// HubbleOption configures the HubbleObserver.
type HubbleOption func(*HubbleObserver)

// WithHubbleDialOptions overrides the default gRPC dial options.
func WithHubbleDialOptions(opts ...grpc.DialOption) HubbleOption {
	return func(o *HubbleObserver) {
		o.dialOpts = opts
	}
}

// WithHubbleFlowWindow sets the time window for GetFlows queries.
func WithHubbleFlowWindow(d time.Duration) HubbleOption {
	return func(o *HubbleObserver) {
		o.flowWindow = d
	}
}

// NewHubbleObserver creates a new Hubble-based flow observer.
func NewHubbleObserver(address string, opts ...HubbleOption) *HubbleObserver {
	if address == "" {
		address = defaultHubbleAddress
	}

	o := &HubbleObserver{
		address:    address,
		flowWindow: defaultFlowWindow,
		dialOpts:   []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

// Available checks if the Hubble Relay is reachable via gRPC.
func (o *HubbleObserver) Available(ctx context.Context) (bool, error) {
	dialCtx, cancel := context.WithTimeout(ctx, hubbleDialTimeout)
	defer cancel()

	conn, err := grpc.NewClient(o.address, o.dialOpts...)
	if err != nil {
		return false, nil
	}

	client := observerpb.NewObserverClient(conn)

	_, err = client.ServerStatus(dialCtx, &observerpb.ServerStatusRequest{})
	if err != nil {
		_ = conn.Close()
		return false, nil
	}

	o.mu.Lock()
	o.conn = conn
	o.mu.Unlock()

	return true, nil
}

// LastSeen queries Hubble for recently forwarded flows and matches them to edges.
func (o *HubbleObserver) LastSeen(ctx context.Context, edges []domainnetpol.FlowEdge) (map[string]time.Time, error) {
	o.mu.Lock()
	conn := o.conn
	o.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("hubble not connected")
	}

	// Build lookup: pairKey ("srcNs/srcName→dstNs/dstName") → list of edge keys.
	type pairKey struct{ srcNs, srcName, dstNs, dstName string }

	pairToEdgeKeys := make(map[pairKey][]string)

	for _, e := range edges {
		_, srcNs, srcName := ParseNodeID(e.From)
		_, dstNs, dstName := ParseNodeID(e.To)

		if srcNs == "" || dstNs == "" {
			continue
		}

		pk := pairKey{srcNs: srcNs, srcName: srcName, dstNs: dstNs, dstName: dstName}
		pairToEdgeKeys[pk] = append(pairToEdgeKeys[pk], domainnetpol.EdgeKey(e))
	}

	if len(pairToEdgeKeys) == 0 {
		return nil, nil
	}

	// Query Hubble for recent forwarded flows.
	client := observerpb.NewObserverClient(conn)
	since := time.Now().Add(-o.flowWindow)

	stream, err := client.GetFlows(ctx, &observerpb.GetFlowsRequest{
		Since:  timestamppb.New(since),
		Follow: false,
		Whitelist: []*flowpb.FlowFilter{
			{Verdict: []flowpb.Verdict{flowpb.Verdict_FORWARDED}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("hubble GetFlows: %w", err)
	}

	lastSeen := make(map[string]time.Time)

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			// Partial results are fine — return what we have.
			break
		}

		flow := resp.GetFlow()
		if flow == nil {
			continue
		}

		src := flow.GetSource()
		dst := flow.GetDestination()

		if src == nil || dst == nil {
			continue
		}

		srcNs := src.GetNamespace()
		srcName := workloadName(src)
		dstNs := dst.GetNamespace()
		dstName := workloadName(dst)

		if srcNs == "" || srcName == "" || dstNs == "" || dstName == "" {
			continue
		}

		pk := pairKey{srcNs: srcNs, srcName: srcName, dstNs: dstNs, dstName: dstName}

		edgeKeys, ok := pairToEdgeKeys[pk]
		if !ok {
			continue
		}

		flowTime := flow.GetTime().AsTime()

		for _, key := range edgeKeys {
			if existing, ok := lastSeen[key]; !ok || flowTime.After(existing) {
				lastSeen[key] = flowTime
			}
		}
	}

	return lastSeen, nil
}

// workloadName extracts the workload name from a Hubble Endpoint.
// Prefers Workloads[0].Name, falls back to pod name labels.
func workloadName(ep *flowpb.Endpoint) string {
	if wl := ep.GetWorkloads(); len(wl) > 0 && wl[0].GetName() != "" {
		return wl[0].GetName()
	}

	// Fallback: use labels for app name.
	for _, label := range ep.GetLabels() {
		// Cilium encodes K8s labels as "k8s:key=value".
		if len(label) > 4 && label[:4] == "k8s:" {
			kv := label[4:]
			for _, prefix := range []string{"app.kubernetes.io/name=", "app="} {
				if len(kv) > len(prefix) && kv[:len(prefix)] == prefix {
					return kv[len(prefix):]
				}
			}
		}
	}

	return ""
}
