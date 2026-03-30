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

package grpc_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	dnsv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	dnsstore "github.com/golgoth31/sreportal/internal/readstore/dns"
)

func seedFQDNStore(t *testing.T) *dnsstore.FQDNStore {
	t.Helper()
	store := dnsstore.NewFQDNStore(nil)
	ctx := context.Background()

	now := time.Now()
	ref, _ := domaindns.ParseResourceRef("service/production/api-svc")

	err := store.Replace(ctx, "default/test-dns", []domaindns.FQDNView{
		{
			Name: "api.example.com", Source: domaindns.SourceExternalDNS,
			Groups: []string{"Services"}, RecordType: "A",
			Targets: []string{"10.0.0.1"}, LastSeen: now,
			PortalName: "main", Namespace: "default", SyncStatus: "synced",
			OriginRef: &ref,
		},
		{
			Name: "web.example.com", Source: domaindns.SourceExternalDNS,
			Groups: []string{"Services"}, RecordType: "A",
			Targets: []string{"10.0.0.2"}, LastSeen: now,
			PortalName: "main", Namespace: "default",
		},
		{
			Name: "internal.example.com", Source: domaindns.SourceManual,
			Groups: []string{"Internal"}, RecordType: "A",
			Targets: []string{"10.0.0.3"}, LastSeen: now,
			PortalName: "main", Namespace: "default",
		},
	})
	require.NoError(t, err)

	return store
}

func TestListFQDNs_ReturnsAllFQDNs(t *testing.T) {
	store := seedFQDNStore(t)
	svc := svcgrpc.NewDNSService(store, nil)

	resp, err := svc.ListFQDNs(
		context.Background(),
		connect.NewRequest(&dnsv1.ListFQDNsRequest{}),
	)

	require.NoError(t, err)
	require.Len(t, resp.Msg.Fqdns, 3)

	fqdnsByName := make(map[string]*dnsv1.FQDN, len(resp.Msg.Fqdns))
	for _, f := range resp.Msg.Fqdns {
		fqdnsByName[f.Name] = f
	}

	assert.Contains(t, fqdnsByName, "api.example.com")
	assert.Equal(t, "external-dns", fqdnsByName["api.example.com"].Source)
	assert.Contains(t, fqdnsByName, "internal.example.com")
	assert.Equal(t, "manual", fqdnsByName["internal.example.com"].Source)
}

func TestListFQDNs_NoDuplicateGroups(t *testing.T) {
	store := seedFQDNStore(t)
	svc := svcgrpc.NewDNSService(store, nil)

	resp, err := svc.ListFQDNs(
		context.Background(),
		connect.NewRequest(&dnsv1.ListFQDNsRequest{}),
	)

	require.NoError(t, err)
	for _, fqdn := range resp.Msg.Fqdns {
		if fqdn.Name == "api.example.com" {
			assert.Len(t, fqdn.Groups, 1,
				"api.example.com should have exactly 1 group, got %v", fqdn.Groups)
			assert.Equal(t, "Services", fqdn.Groups[0])
		}
	}
}

func TestListFQDNs_OriginRef_IsPopulated(t *testing.T) {
	store := seedFQDNStore(t)
	svc := svcgrpc.NewDNSService(store, nil)

	resp, err := svc.ListFQDNs(
		context.Background(),
		connect.NewRequest(&dnsv1.ListFQDNsRequest{}),
	)

	require.NoError(t, err)

	var apiFQDN *dnsv1.FQDN
	for _, f := range resp.Msg.Fqdns {
		if f.Name == "api.example.com" {
			apiFQDN = f
			break
		}
	}

	require.NotNil(t, apiFQDN)
	require.NotNil(t, apiFQDN.OriginRef, "OriginRef must be set for external-dns FQDNs")
	assert.Equal(t, "service", apiFQDN.OriginRef.Kind)
	assert.Equal(t, "production", apiFQDN.OriginRef.Namespace)
	assert.Equal(t, "api-svc", apiFQDN.OriginRef.Name)
}

func TestListFQDNs_OriginRef_IsNil_ForManualEntries(t *testing.T) {
	store := seedFQDNStore(t)
	svc := svcgrpc.NewDNSService(store, nil)

	resp, err := svc.ListFQDNs(
		context.Background(),
		connect.NewRequest(&dnsv1.ListFQDNsRequest{}),
	)

	require.NoError(t, err)

	for _, f := range resp.Msg.Fqdns {
		if f.Name == "internal.example.com" {
			assert.Nil(t, f.OriginRef, "manual entries must not have OriginRef")
		}
	}
}

func TestListFQDNs_ReturnsBothRecordTypes(t *testing.T) {
	store := dnsstore.NewFQDNStore(nil)
	now := time.Now()
	_ = store.Replace(context.Background(), "default/test-dns", []domaindns.FQDNView{
		{Name: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, LastSeen: now, PortalName: "main"},
		{Name: "api.example.com", RecordType: "CNAME", Targets: []string{"lb.example.com"}, LastSeen: now, PortalName: "main"},
	})

	svc := svcgrpc.NewDNSService(store, nil)

	resp, err := svc.ListFQDNs(
		context.Background(),
		connect.NewRequest(&dnsv1.ListFQDNsRequest{}),
	)

	require.NoError(t, err)
	require.Len(t, resp.Msg.Fqdns, 2, "A and CNAME for the same hostname must both be returned")

	recordTypes := make(map[string]string)
	for _, f := range resp.Msg.Fqdns {
		assert.Equal(t, "api.example.com", f.Name)
		recordTypes[f.RecordType] = f.Targets[0]
	}
	assert.Equal(t, "10.0.0.1", recordTypes["A"])
	assert.Equal(t, "lb.example.com", recordTypes["CNAME"])
}

func TestListFQDNs_FiltersWork(t *testing.T) {
	store := seedFQDNStore(t)
	svc := svcgrpc.NewDNSService(store, nil)

	cases := []struct {
		name     string
		request  *dnsv1.ListFQDNsRequest
		wantLen  int
		wantFQDN string
	}{
		{
			name:     "search filter",
			request:  &dnsv1.ListFQDNsRequest{Search: "api"},
			wantLen:  1,
			wantFQDN: "api.example.com",
		},
		{
			name:     "source filter external-dns",
			request:  &dnsv1.ListFQDNsRequest{Source: "external-dns"},
			wantLen:  2,
			wantFQDN: "api.example.com",
		},
		{
			name:     "source filter manual",
			request:  &dnsv1.ListFQDNsRequest{Source: "manual"},
			wantLen:  1,
			wantFQDN: "internal.example.com",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := svc.ListFQDNs(
				context.Background(),
				connect.NewRequest(tc.request),
			)
			require.NoError(t, err)
			assert.Len(t, resp.Msg.Fqdns, tc.wantLen)

			var found bool
			for _, f := range resp.Msg.Fqdns {
				if f.Name == tc.wantFQDN {
					found = true
					break
				}
			}
			assert.True(t, found, "expected to find FQDN %q", tc.wantFQDN)
		})
	}
}

func TestListFQDNs_TotalSize_ReflectsFullCount(t *testing.T) {
	store := seedFQDNStore(t)
	svc := svcgrpc.NewDNSService(store, nil)

	resp, err := svc.ListFQDNs(
		context.Background(),
		connect.NewRequest(&dnsv1.ListFQDNsRequest{}),
	)

	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.Msg.TotalSize)
}
