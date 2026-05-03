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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	portalstore "github.com/golgoth31/sreportal/internal/readstore/portal"
)

func TestIsFeatureEnabled(t *testing.T) {
	ctx := context.Background()

	portalWithFeatures := func(name string, dns, releases, netpol, alerts, statusPage bool) domainportal.PortalView {
		return domainportal.PortalView{
			Name: name,
			Features: domainportal.PortalFeatures{
				DNS:           dns,
				Releases:      releases,
				NetworkPolicy: netpol,
				Alerts:        alerts,
				StatusPage:    statusPage,
			},
		}
	}

	tests := []struct {
		name       string
		portalName string
		checker    svcgrpc.FeatureChecker
		portals    []domainportal.PortalView
		nilReader  bool
		want       bool
	}{
		{
			name:      "nil reader returns true",
			nilReader: true,
			want:      true,
		},
		{
			name:       "empty portal name returns true",
			portalName: "",
			checker:    svcgrpc.CheckDNS,
			want:       true,
		},
		{
			name:       "unknown portal returns true",
			portalName: "unknown",
			checker:    svcgrpc.CheckDNS,
			portals:    []domainportal.PortalView{portalWithFeatures("other", true, true, true, true, true)},
			want:       true,
		},
		{
			name:       "DNS enabled",
			portalName: tPortalMyPortal,
			checker:    svcgrpc.CheckDNS,
			portals:    []domainportal.PortalView{portalWithFeatures(tPortalMyPortal, true, true, true, true, true)},
			want:       true,
		},
		{
			name:       "DNS disabled",
			portalName: tPortalMyPortal,
			checker:    svcgrpc.CheckDNS,
			portals:    []domainportal.PortalView{portalWithFeatures(tPortalMyPortal, false, true, true, true, true)},
			want:       false,
		},
		{
			name:       "alerts disabled",
			portalName: tPortalMyPortal,
			checker:    svcgrpc.CheckAlerts,
			portals:    []domainportal.PortalView{portalWithFeatures(tPortalMyPortal, true, true, true, false, true)},
			want:       false,
		},
		{
			name:       "networkPolicy disabled",
			portalName: tPortalMyPortal,
			checker:    svcgrpc.CheckNetworkPolicy,
			portals:    []domainportal.PortalView{portalWithFeatures(tPortalMyPortal, true, true, false, true, true)},
			want:       false,
		},
		{
			name:       "releases disabled",
			portalName: tPortalMyPortal,
			checker:    svcgrpc.CheckReleases,
			portals:    []domainportal.PortalView{portalWithFeatures(tPortalMyPortal, true, false, true, true, true)},
			want:       false,
		},
		{
			name:       "statusPage disabled",
			portalName: tPortalMyPortal,
			checker:    svcgrpc.CheckStatusPage,
			portals:    []domainportal.PortalView{portalWithFeatures(tPortalMyPortal, true, true, true, true, false)},
			want:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var reader domainportal.PortalReader
			if !tc.nilReader {
				store := portalstore.NewPortalStore()
				for _, p := range tc.portals {
					require.NoError(t, store.Replace(ctx, p.Name, p))
				}
				reader = store
			}

			got, err := svcgrpc.IsFeatureEnabled(ctx, reader, tc.portalName, tc.checker)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// errorPortalReader is a fake that always returns an error.
type errorPortalReader struct{}

func (e *errorPortalReader) List(_ context.Context, _ domainportal.PortalFilters) ([]domainportal.PortalView, error) {
	return nil, fmt.Errorf("store unavailable")
}

func (e *errorPortalReader) Subscribe() <-chan struct{} {
	return make(chan struct{})
}

func TestIsFeatureEnabled_ReaderError(t *testing.T) {
	_, err := svcgrpc.IsFeatureEnabled(context.Background(), &errorPortalReader{}, "some-portal", svcgrpc.CheckDNS)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store unavailable")
}
