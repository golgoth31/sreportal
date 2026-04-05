package flowobserver

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	flowpb "github.com/golgoth31/sreportal/internal/flowobserver/hubblegen/flow"
	observerpb "github.com/golgoth31/sreportal/internal/flowobserver/hubblegen/observer"
)

const bufSize = 1024 * 1024

// fakeObserverServer implements the Observer gRPC service for testing.
type fakeObserverServer struct {
	observerpb.UnimplementedObserverServer
	flows []*flowpb.Flow
}

func (s *fakeObserverServer) ServerStatus(_ context.Context, _ *observerpb.ServerStatusRequest) (*observerpb.ServerStatusResponse, error) {
	return &observerpb.ServerStatusResponse{NumFlows: 100, MaxFlows: 4096}, nil
}

func (s *fakeObserverServer) GetFlows(_ *observerpb.GetFlowsRequest, stream grpc.ServerStreamingServer[observerpb.GetFlowsResponse]) error {
	for _, f := range s.flows {
		if err := stream.Send(&observerpb.GetFlowsResponse{
			ResponseTypes: &observerpb.GetFlowsResponse_Flow{Flow: f},
			Time:          f.GetTime(),
		}); err != nil {
			return err
		}
	}

	return nil
}

func newTestHubble(t *testing.T, srv *fakeObserverServer) *HubbleObserver {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	observerpb.RegisterObserverServer(s, srv)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("server exited: %v", err)
		}
	}()

	t.Cleanup(func() {
		s.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}

	obs := &HubbleObserver{address: "bufconn"}
	obs.conn = conn

	return obs
}

func makeFlow(srcNs, srcName, dstNs, dstName string, ts time.Time) *flowpb.Flow {
	return &flowpb.Flow{
		Time:    timestamppb.New(ts),
		Verdict: flowpb.Verdict_FORWARDED,
		Source: &flowpb.Endpoint{
			Namespace: srcNs,
			Workloads: []*flowpb.Workload{{Name: srcName}},
		},
		Destination: &flowpb.Endpoint{
			Namespace: dstNs,
			Workloads: []*flowpb.Workload{{Name: dstName}},
		},
	}
}

func TestHubbleObserver_LastSeen_MatchesEdges(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	srv := &fakeObserverServer{
		flows: []*flowpb.Flow{
			makeFlow("core", "api-server", "data", "postgres", now),
			makeFlow("web", "frontend", "core", "api-server", now.Add(-2*time.Minute)),
			makeFlow("other", "unrelated", "other", "service", now), // not in our edges
		},
	}

	obs := newTestHubble(t, srv)

	edges := []domainnetpol.FlowEdge{
		{From: "service:core:api-server", To: "database:data:postgres", EdgeType: "database"},
		{From: "service:web:frontend", To: "service:core:api-server", EdgeType: "cross-ns"},
		{From: "service:core:api-server", To: "service:core:unknown", EdgeType: "internal"}, // no match
	}

	result, err := obs.LastSeen(context.Background(), edges)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(result), result)
	}

	key1 := domainnetpol.EdgeKey(edges[0])
	if ts, ok := result[key1]; !ok {
		t.Errorf("expected match for %s", key1)
	} else if !ts.Equal(now) {
		t.Errorf("expected timestamp %v for %s, got %v", now, key1, ts)
	}

	key2 := domainnetpol.EdgeKey(edges[1])
	if _, ok := result[key2]; !ok {
		t.Errorf("expected match for %s", key2)
	}

	key3 := domainnetpol.EdgeKey(edges[2])
	if _, ok := result[key3]; ok {
		t.Errorf("did not expect match for %s", key3)
	}
}

func TestHubbleObserver_LastSeen_EmptyStream(t *testing.T) {
	srv := &fakeObserverServer{flows: nil}
	obs := newTestHubble(t, srv)

	edges := []domainnetpol.FlowEdge{
		{From: "service:a:b", To: "service:c:d", EdgeType: "internal"},
	}

	result, err := obs.LastSeen(context.Background(), edges)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 0 {
		t.Fatalf("expected 0 matches for empty stream, got %d", len(result))
	}
}

func TestHubbleObserver_LastSeen_KeepsLatestTimestamp(t *testing.T) {
	earlier := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	later := time.Now().Add(-1 * time.Minute).Truncate(time.Second)

	srv := &fakeObserverServer{
		flows: []*flowpb.Flow{
			makeFlow("core", "api", "data", "db", earlier),
			makeFlow("core", "api", "data", "db", later),
		},
	}

	obs := newTestHubble(t, srv)

	edges := []domainnetpol.FlowEdge{
		{From: "service:core:api", To: "database:data:db", EdgeType: "database"},
	}

	result, err := obs.LastSeen(context.Background(), edges)
	if err != nil {
		t.Fatal(err)
	}

	key := domainnetpol.EdgeKey(edges[0])
	if ts, ok := result[key]; !ok {
		t.Fatal("expected match")
	} else if !ts.Equal(later) {
		t.Errorf("expected latest timestamp %v, got %v", later, ts)
	}
}
