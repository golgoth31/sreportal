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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
)

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
	Value  [2]any            `json:"value"`
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

func testSpec(address string) sreportalv1alpha1.FlowObserverSpec {
	return sreportalv1alpha1.FlowObserverSpec{
		Prometheus: sreportalv1alpha1.FlowObserverPrometheusConfig{
			Address:     address,
			QueryWindow: "5m",
		},
		// Empty metrics → uses defaults.
	}
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

	obs, err := NewPrometheusObserver(testSpec(srv.URL))
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

	if obs.ActiveMesh() != "istio" {
		t.Fatalf("expected mesh 'istio', got %q", obs.ActiveMesh())
	}
}

func TestPrometheusObserver_Available_NoMetrics(t *testing.T) {
	srv := newPromServer(t, func(_ string) promResponse {
		return emptyVector()
	})
	defer srv.Close()

	obs, err := NewPrometheusObserver(testSpec(srv.URL))
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

func TestPrometheusObserver_Observed_MatchesEdges(t *testing.T) {
	now := time.Now()

	srv := newPromServer(t, func(query string) promResponse {
		if query == `count(hubble_flows_processed_total)` {
			return emptyVector()
		}
		if query == `count(istio_requests_total)` {
			return vectorWith(promSample{Value: [2]any{float64(now.Unix()), "1"}})
		}

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

	obs, err := NewPrometheusObserver(testSpec(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := obs.Available(context.Background()); err != nil {
		t.Fatal(err)
	}

	edges := []domainnetpol.FlowEdge{
		{From: "service:core:api-server", To: "database:data:postgres", EdgeType: "database"},
		{From: "service:web:frontend", To: "service:core:api-server", EdgeType: "cross-ns"},
		{From: "service:core:api-server", To: "service:core:unknown", EdgeType: "internal"},
	}

	result, err := obs.Observed(context.Background(), edges)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(result), result)
	}

	key1 := domainnetpol.EdgeKey(edges[0])
	if !result[key1] {
		t.Errorf("expected match for %s", key1)
	}

	key2 := domainnetpol.EdgeKey(edges[1])
	if !result[key2] {
		t.Errorf("expected match for %s", key2)
	}

	key3 := domainnetpol.EdgeKey(edges[2])
	if result[key3] {
		t.Errorf("did not expect match for %s", key3)
	}
}

func TestPrometheusObserver_Observed_NilMesh(t *testing.T) {
	obs := &PrometheusObserver{}

	result, err := obs.Observed(context.Background(), []domainnetpol.FlowEdge{
		{From: "service:a:b", To: "service:c:d", EdgeType: "internal"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}
