package flowobserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
)

// promResponse is a minimal Prometheus API response for testing.
type promResponse struct {
	Status string   `json:"status"`
	Data   promData `json:"data"`
}

type promData struct {
	ResultType string       `json:"resultType"`
	Result     []promSample `json:"result"`
}

type promSample struct {
	Metric map[string]string `json:"metric"`
	Value  [2]any            `json:"value"` // [timestamp, "value"]
}

func newPromServer(t *testing.T, handler func(query string) promResponse) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		query := r.FormValue("query")
		resp := handler(query)
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatal(err)
		}
	})

	return httptest.NewServer(mux)
}

func emptyVector() promResponse {
	return promResponse{Status: "success", Data: promData{ResultType: "vector", Result: nil}}
}

func vectorWith(samples ...promSample) promResponse {
	return promResponse{Status: "success", Data: promData{ResultType: "vector", Result: samples}}
}

func TestPrometheusObserver_Available_DetectsIstio(t *testing.T) {
	srv := newPromServer(t, func(query string) promResponse {
		if query == `count(hubble_flows_processed_total)` {
			return emptyVector()
		}
		if query == `count(istio_requests_total)` {
			return vectorWith(promSample{
				Metric: map[string]string{},
				Value:  [2]any{float64(time.Now().Unix()), "42"},
			})
		}

		return emptyVector()
	})
	defer srv.Close()

	obs, err := NewPrometheusObserver(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := obs.Available(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !ok {
		t.Fatal("expected Available() = true")
	}

	if obs.MeshName() != "istio" {
		t.Fatalf("expected mesh 'istio', got %q", obs.MeshName())
	}
}

func TestPrometheusObserver_Available_NoMetrics(t *testing.T) {
	srv := newPromServer(t, func(_ string) promResponse {
		return emptyVector()
	})
	defer srv.Close()

	obs, err := NewPrometheusObserver(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := obs.Available(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if ok {
		t.Fatal("expected Available() = false when no metrics exist")
	}
}

func TestPrometheusObserver_LastSeen_MatchesEdges(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	srv := newPromServer(t, func(query string) promResponse {
		// Probe queries for detection
		if query == `count(hubble_flows_processed_total)` {
			return emptyVector()
		}
		if query == `count(istio_requests_total)` {
			return vectorWith(promSample{Value: [2]any{float64(now.Unix()), "1"}})
		}
		// LastSeen query — value[0] is scrape timestamp, value[1] is the metric value (from timestamp() function)
		return vectorWith(
			promSample{
				Metric: map[string]string{
					"source_workload":                "api-server",
					"source_workload_namespace":      "core",
					"destination_workload":           "postgres",
					"destination_workload_namespace": "data",
				},
				Value: [2]any{float64(now.Unix()), fmt.Sprintf("%d", now.Unix())},
			},
			promSample{
				Metric: map[string]string{
					"source_workload":                "frontend",
					"source_workload_namespace":      "web",
					"destination_workload":           "api-server",
					"destination_workload_namespace": "core",
				},
				Value: [2]any{float64(now.Unix()), fmt.Sprintf("%d", now.Add(-5*time.Minute).Unix())},
			},
		)
	})
	defer srv.Close()

	obs, err := NewPrometheusObserver(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := obs.Available(context.Background()); err != nil {
		t.Fatal(err)
	}

	edges := []domainnetpol.FlowEdge{
		{From: "service:core:api-server", To: "database:data:postgres", EdgeType: "database"},
		{From: "service:web:frontend", To: "service:core:api-server", EdgeType: "cross-ns"},
		{From: "service:core:api-server", To: "service:core:unknown", EdgeType: "internal"}, // no match
	}

	result, err := obs.LastSeen(context.Background(), edges)
	if err != nil {
		t.Fatal(err)
	}

	// Should match 2 edges
	if len(result) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(result), result)
	}

	key1 := domainnetpol.EdgeKey(edges[0])
	if _, ok := result[key1]; !ok {
		t.Errorf("expected match for %s", key1)
	}

	key2 := domainnetpol.EdgeKey(edges[1])
	if _, ok := result[key2]; !ok {
		t.Errorf("expected match for %s", key2)
	}

	// Should NOT match the third edge
	key3 := domainnetpol.EdgeKey(edges[2])
	if _, ok := result[key3]; ok {
		t.Errorf("did not expect match for %s", key3)
	}
}

func TestPrometheusObserver_LastSeen_NilMesh(t *testing.T) {
	obs := &PrometheusObserver{} // no mesh detected

	result, err := obs.LastSeen(context.Background(), []domainnetpol.FlowEdge{
		{From: "service:a:b", To: "service:c:d", EdgeType: "internal"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}
