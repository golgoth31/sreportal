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

package remoteclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	sreportalv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// mockDNSServiceHandler implements the DNS service for testing.
type mockDNSServiceHandler struct {
	sreportalv1connect.UnimplementedDNSServiceHandler
	fqdns []*sreportalv1.FQDN
	err   error
}

func (m *mockDNSServiceHandler) ListFQDNs(
	_ context.Context,
	_ *connect.Request[sreportalv1.ListFQDNsRequest],
) (*connect.Response[sreportalv1.ListFQDNsResponse], error) {
	if m.err != nil {
		return nil, m.err
	}
	return connect.NewResponse(&sreportalv1.ListFQDNsResponse{
		Fqdns: m.fqdns,
	}), nil
}

// mockPortalServiceHandler implements the Portal service for testing.
type mockPortalServiceHandler struct {
	sreportalv1connect.UnimplementedPortalServiceHandler
	portals []*sreportalv1.Portal
	err     error
}

func (m *mockPortalServiceHandler) ListPortals(
	_ context.Context,
	_ *connect.Request[sreportalv1.ListPortalsRequest],
) (*connect.Response[sreportalv1.ListPortalsResponse], error) {
	if m.err != nil {
		return nil, m.err
	}
	return connect.NewResponse(&sreportalv1.ListPortalsResponse{
		Portals: m.portals,
	}), nil
}

func TestNewClient(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		client := NewClient()
		assert.NotNil(t, client)
		assert.Equal(t, DefaultTimeout, client.timeout)
		assert.Equal(t, DefaultRetryAttempts, client.retryAttempts)
		assert.Equal(t, DefaultRetryDelay, client.retryDelay)
	})

	t.Run("with custom timeout", func(t *testing.T) {
		customTimeout := 30 * time.Second
		client := NewClient(WithTimeout(customTimeout))
		assert.Equal(t, customTimeout, client.timeout)
	})

	t.Run("with custom retry attempts", func(t *testing.T) {
		client := NewClient(WithRetryAttempts(5))
		assert.Equal(t, 5, client.retryAttempts)
	})

	t.Run("with custom retry delay", func(t *testing.T) {
		customDelay := 1 * time.Second
		client := NewClient(WithRetryDelay(customDelay))
		assert.Equal(t, customDelay, client.retryDelay)
	})

	t.Run("with custom HTTP client", func(t *testing.T) {
		customHTTPClient := &http.Client{Timeout: 60 * time.Second}
		client := NewClient(WithHTTPClient(customHTTPClient))
		assert.Equal(t, customHTTPClient, client.httpClient)
	})
}

func TestFetchFQDNs(t *testing.T) {
	t.Run("successful fetch with FQDNs", func(t *testing.T) {
		now := time.Now()
		dnsHandler := &mockDNSServiceHandler{
			fqdns: []*sreportalv1.FQDN{
				{
					Name:        "app.example.com",
					Description: "Application endpoint",
					RecordType:  "A",
					Targets:     []string{"192.168.1.1"},
					Groups:      []string{"production"},
					LastSeen:    timestamppb.New(now),
				},
				{
					Name:        "api.example.com",
					Description: "API endpoint",
					RecordType:  "CNAME",
					Targets:     []string{"lb.example.com"},
					Groups:      []string{"production"},
					LastSeen:    timestamppb.New(now),
				},
				{
					Name:        "dev.example.com",
					Description: "Dev endpoint",
					RecordType:  "A",
					Targets:     []string{"10.0.0.1"},
					Groups:      []string{"development"},
					LastSeen:    timestamppb.New(now),
				},
			},
		}

		portalHandler := &mockPortalServiceHandler{
			portals: []*sreportalv1.Portal{
				{
					Name:  "main",
					Title: "Main Portal",
					Main:  true,
				},
			},
		}

		mux := http.NewServeMux()
		mux.Handle(sreportalv1connect.NewDNSServiceHandler(dnsHandler))
		mux.Handle(sreportalv1connect.NewPortalServiceHandler(portalHandler))

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient(WithRetryAttempts(1))
		result, err := client.FetchFQDNs(context.Background(), server.URL, "")

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 3, result.FQDNCount)
		assert.Equal(t, "Main Portal", result.RemoteTitle)
		assert.Len(t, result.Groups, 2) // production and development groups
	})

	t.Run("successful fetch with empty response", func(t *testing.T) {
		dnsHandler := &mockDNSServiceHandler{
			fqdns: []*sreportalv1.FQDN{},
		}

		portalHandler := &mockPortalServiceHandler{
			portals: []*sreportalv1.Portal{},
		}

		mux := http.NewServeMux()
		mux.Handle(sreportalv1connect.NewDNSServiceHandler(dnsHandler))
		mux.Handle(sreportalv1connect.NewPortalServiceHandler(portalHandler))

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient(WithRetryAttempts(1))
		result, err := client.FetchFQDNs(context.Background(), server.URL, "")

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 0, result.FQDNCount)
		assert.Empty(t, result.Groups)
	})

	t.Run("fetch with specific portal name", func(t *testing.T) {
		dnsHandler := &mockDNSServiceHandler{
			fqdns: []*sreportalv1.FQDN{
				{
					Name:       "app.example.com",
					RecordType: "A",
					Targets:    []string{"192.168.1.1"},
					Groups:     []string{"default"},
				},
			},
		}

		portalHandler := &mockPortalServiceHandler{
			portals: []*sreportalv1.Portal{
				{
					Name:  "main",
					Title: "Main Portal",
					Main:  true,
				},
				{
					Name:  "secondary",
					Title: "Secondary Portal",
					Main:  false,
				},
			},
		}

		mux := http.NewServeMux()
		mux.Handle(sreportalv1connect.NewDNSServiceHandler(dnsHandler))
		mux.Handle(sreportalv1connect.NewPortalServiceHandler(portalHandler))

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient(WithRetryAttempts(1))
		result, err := client.FetchFQDNs(context.Background(), server.URL, "secondary")

		require.NoError(t, err)
		assert.Equal(t, "Secondary Portal", result.RemoteTitle)
	})

	t.Run("error when server is unavailable", func(t *testing.T) {
		client := NewClient(
			WithRetryAttempts(1),
			WithTimeout(100*time.Millisecond),
		)
		_, err := client.FetchFQDNs(context.Background(), "http://localhost:59999", "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed after 1 attempts")
	})

	t.Run("error from DNS service", func(t *testing.T) {
		dnsHandler := &mockDNSServiceHandler{
			err: connect.NewError(connect.CodeInternal, assert.AnError),
		}

		portalHandler := &mockPortalServiceHandler{
			portals: []*sreportalv1.Portal{},
		}

		mux := http.NewServeMux()
		mux.Handle(sreportalv1connect.NewDNSServiceHandler(dnsHandler))
		mux.Handle(sreportalv1connect.NewPortalServiceHandler(portalHandler))

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient(WithRetryAttempts(1))
		_, err := client.FetchFQDNs(context.Background(), server.URL, "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch FQDNs from remote portal")
	})

	t.Run("context cancellation", func(t *testing.T) {
		dnsHandler := &mockDNSServiceHandler{
			fqdns: []*sreportalv1.FQDN{},
		}

		portalHandler := &mockPortalServiceHandler{
			portals: []*sreportalv1.Portal{},
		}

		mux := http.NewServeMux()
		mux.Handle(sreportalv1connect.NewDNSServiceHandler(dnsHandler))
		mux.Handle(sreportalv1connect.NewPortalServiceHandler(portalHandler))

		server := httptest.NewServer(mux)
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		client := NewClient(WithRetryAttempts(3), WithRetryDelay(1*time.Second))
		_, err := client.FetchFQDNs(ctx, server.URL, "")

		require.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestHealthCheck(t *testing.T) {
	t.Run("successful health check", func(t *testing.T) {
		portalHandler := &mockPortalServiceHandler{
			portals: []*sreportalv1.Portal{
				{
					Name:  "main",
					Title: "Main Portal",
					Main:  true,
				},
			},
		}

		mux := http.NewServeMux()
		mux.Handle(sreportalv1connect.NewPortalServiceHandler(portalHandler))

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient()
		err := client.HealthCheck(context.Background(), server.URL)

		require.NoError(t, err)
	})

	t.Run("failed health check - server unavailable", func(t *testing.T) {
		client := NewClient(WithTimeout(100 * time.Millisecond))
		err := client.HealthCheck(context.Background(), "http://localhost:59999")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "health check failed")
	})

	t.Run("failed health check - service error", func(t *testing.T) {
		portalHandler := &mockPortalServiceHandler{
			err: connect.NewError(connect.CodeUnavailable, assert.AnError),
		}

		mux := http.NewServeMux()
		mux.Handle(sreportalv1connect.NewPortalServiceHandler(portalHandler))

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient()
		err := client.HealthCheck(context.Background(), server.URL)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "health check failed")
	})
}

func TestConvertToGroups(t *testing.T) {
	t.Run("converts FQDNs to groups correctly", func(t *testing.T) {
		now := time.Now()
		fqdns := []*sreportalv1.FQDN{
			{
				Name:        "app1.example.com",
				Description: "App 1",
				RecordType:  "A",
				Targets:     []string{"192.168.1.1"},
				Groups:      []string{"production"},
				LastSeen:    timestamppb.New(now),
			},
			{
				Name:        "app2.example.com",
				Description: "App 2",
				RecordType:  "A",
				Targets:     []string{"192.168.1.2"},
				Groups:      []string{"production"},
				LastSeen:    timestamppb.New(now),
			},
			{
				Name:        "dev.example.com",
				Description: "Dev",
				RecordType:  "A",
				Targets:     []string{"10.0.0.1"},
				Groups:      []string{"development"},
				LastSeen:    timestamppb.New(now),
			},
		}

		groups := convertToGroups(fqdns)

		assert.Len(t, groups, 2)

		// Find groups by name
		var prodGroup, devGroup *struct {
			name   string
			fqdns  int
			source string
		}
		for _, g := range groups {
			if g.Name == "production" {
				prodGroup = &struct {
					name   string
					fqdns  int
					source string
				}{g.Name, len(g.FQDNs), g.Source}
			}
			if g.Name == "development" {
				devGroup = &struct {
					name   string
					fqdns  int
					source string
				}{g.Name, len(g.FQDNs), g.Source}
			}
		}

		require.NotNil(t, prodGroup)
		assert.Equal(t, 2, prodGroup.fqdns)
		assert.Equal(t, "remote", prodGroup.source)

		require.NotNil(t, devGroup)
		assert.Equal(t, 1, devGroup.fqdns)
		assert.Equal(t, "remote", devGroup.source)
	})

	t.Run("uses default group for empty group name", func(t *testing.T) {
		fqdns := []*sreportalv1.FQDN{
			{
				Name:       "app.example.com",
				RecordType: "A",
				Targets:    []string{"192.168.1.1"},
				Groups:     []string{}, // Empty groups
			},
		}

		groups := convertToGroups(fqdns)

		assert.Len(t, groups, 1)
		assert.Equal(t, "default", groups[0].Name)
	})

	t.Run("handles empty input", func(t *testing.T) {
		groups := convertToGroups([]*sreportalv1.FQDN{})
		assert.Empty(t, groups)
	})

	t.Run("handles nil LastSeen", func(t *testing.T) {
		fqdns := []*sreportalv1.FQDN{
			{
				Name:       "app.example.com",
				RecordType: "A",
				Targets:    []string{"192.168.1.1"},
				Groups:     []string{"default"},
				LastSeen:   nil,
			},
		}

		groups := convertToGroups(fqdns)

		assert.Len(t, groups, 1)
		assert.Len(t, groups[0].FQDNs, 1)
		// LastSeen should be set to approximately now
		assert.WithinDuration(t, time.Now(), groups[0].FQDNs[0].LastSeen.Time, 5*time.Second)
	})
}
