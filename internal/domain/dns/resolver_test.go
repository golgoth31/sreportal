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

package dns_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeResolver implements dns.Resolver for testing.
type fakeResolver struct {
	hosts    map[string][]string
	cnames   map[string]string
	hostErr  map[string]error
	cnameErr map[string]error
}

func newFakeResolver() *fakeResolver {
	return &fakeResolver{
		hosts:    make(map[string][]string),
		cnames:   make(map[string]string),
		hostErr:  make(map[string]error),
		cnameErr: make(map[string]error),
	}
}

func (r *fakeResolver) LookupHost(ctx context.Context, fqdn string) ([]string, error) {
	if err, ok := r.hostErr[fqdn]; ok {
		return nil, err
	}
	addrs, ok := r.hosts[fqdn]
	if !ok {
		return nil, &dnsNotFoundError{fqdn: fqdn}
	}
	return addrs, nil
}

func (r *fakeResolver) LookupCNAME(ctx context.Context, fqdn string) (string, error) {
	if err, ok := r.cnameErr[fqdn]; ok {
		return "", err
	}
	cname, ok := r.cnames[fqdn]
	if !ok {
		return "", &dnsNotFoundError{fqdn: fqdn}
	}
	return cname, nil
}

// dnsNotFoundError simulates a DNS not-found error.
type dnsNotFoundError struct {
	fqdn string
}

func (e *dnsNotFoundError) Error() string {
	return "no such host: " + e.fqdn
}

func (e *dnsNotFoundError) NotFound() bool {
	return true
}

func TestCheckFQDN(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name       string
		fqdn       string
		recordType string
		targets    []string
		setup      func(r *fakeResolver)
		wantStatus dns.SyncStatus
	}{
		{
			name:       "A record sync — targets match",
			fqdn:       "app.example.com",
			recordType: "A",
			targets:    []string{"10.0.0.1", "10.0.0.2"},
			setup: func(r *fakeResolver) {
				r.hosts["app.example.com"] = []string{"10.0.0.2", "10.0.0.1"}
			},
			wantStatus: dns.SyncStatusSync,
		},
		{
			name:       "A record notsync — different targets",
			fqdn:       "app.example.com",
			recordType: "A",
			targets:    []string{"10.0.0.1"},
			setup: func(r *fakeResolver) {
				r.hosts["app.example.com"] = []string{"10.0.0.99"}
			},
			wantStatus: dns.SyncStatusNotSync,
		},
		{
			name:       "A record notavailable — NXDOMAIN",
			fqdn:       "gone.example.com",
			recordType: "A",
			targets:    []string{"10.0.0.1"},
			setup:      func(r *fakeResolver) {},
			wantStatus: dns.SyncStatusNotAvailable,
		},
		{
			name:       "AAAA record sync — targets match",
			fqdn:       "ipv6.example.com",
			recordType: "AAAA",
			targets:    []string{"::1", "fe80::1"},
			setup: func(r *fakeResolver) {
				r.hosts["ipv6.example.com"] = []string{"fe80::1", "::1"}
			},
			wantStatus: dns.SyncStatusSync,
		},
		{
			name:       "CNAME record sync — target matches",
			fqdn:       "alias.example.com",
			recordType: "CNAME",
			targets:    []string{"real.example.com."},
			setup: func(r *fakeResolver) {
				r.cnames["alias.example.com"] = "real.example.com."
			},
			wantStatus: dns.SyncStatusSync,
		},
		{
			name:       "CNAME record notsync — different target",
			fqdn:       "alias.example.com",
			recordType: "CNAME",
			targets:    []string{"real.example.com."},
			setup: func(r *fakeResolver) {
				r.cnames["alias.example.com"] = "other.example.com."
			},
			wantStatus: dns.SyncStatusNotSync,
		},
		{
			name:       "CNAME record notavailable — not found",
			fqdn:       "alias.example.com",
			recordType: "CNAME",
			targets:    []string{"real.example.com."},
			setup:      func(r *fakeResolver) {},
			wantStatus: dns.SyncStatusNotAvailable,
		},
		{
			name:       "manual entry — no recordType, no targets, host resolves",
			fqdn:       "manual.example.com",
			recordType: "",
			targets:    nil,
			setup: func(r *fakeResolver) {
				r.hosts["manual.example.com"] = []string{"10.0.0.1"}
			},
			wantStatus: dns.SyncStatusSync,
		},
		{
			name:       "manual entry — no recordType, no targets, host not found",
			fqdn:       "manual.example.com",
			recordType: "",
			targets:    nil,
			setup:      func(r *fakeResolver) {},
			wantStatus: dns.SyncStatusNotAvailable,
		},
		{
			name:       "A record sync — single target",
			fqdn:       "single.example.com",
			recordType: "A",
			targets:    []string{"10.0.0.1"},
			setup: func(r *fakeResolver) {
				r.hosts["single.example.com"] = []string{"10.0.0.1"}
			},
			wantStatus: dns.SyncStatusSync,
		},
		{
			name:       "A record notsync — extra target in DNS",
			fqdn:       "extra.example.com",
			recordType: "A",
			targets:    []string{"10.0.0.1"},
			setup: func(r *fakeResolver) {
				r.hosts["extra.example.com"] = []string{"10.0.0.1", "10.0.0.2"}
			},
			wantStatus: dns.SyncStatusNotSync,
		},
		{
			name:       "A record notsync — missing target in DNS",
			fqdn:       "missing.example.com",
			recordType: "A",
			targets:    []string{"10.0.0.1", "10.0.0.2"},
			setup: func(r *fakeResolver) {
				r.hosts["missing.example.com"] = []string{"10.0.0.1"}
			},
			wantStatus: dns.SyncStatusNotSync,
		},
		{
			name:       "A record notavailable — resolver returns generic error",
			fqdn:       "err.example.com",
			recordType: "A",
			targets:    []string{"10.0.0.1"},
			setup: func(r *fakeResolver) {
				r.hostErr["err.example.com"] = errors.New("temporary DNS failure")
			},
			wantStatus: dns.SyncStatusNotAvailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := newFakeResolver()
			tc.setup(resolver)

			result := dns.CheckFQDN(ctx, resolver, tc.fqdn, tc.recordType, tc.targets)

			require.NotNil(t, result)
			assert.Equal(t, tc.wantStatus, result.Status, "unexpected SyncStatus")
		})
	}
}
