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
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

// TestFQDNValidationMatchesCRD guards the duplication between the domain
// validators (domaindns.FQDNPattern / domaindns.ValidRecordTypes) and the
// generated DNSRecord CRD constraints. The DNS controller pre-filters entries
// with the domain copies; if they ever drift from the CRD the whole "skip one
// bad entry instead of aborting the reconcile" fix silently breaks (entries
// pass the pre-filter but get rejected at admission).
func TestFQDNValidationMatchesCRD(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// internal/domain/dns -> repo root.
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	crdPath := filepath.Join(root, "config", "crd", "bases", "sreportal.io_dnsrecords.yaml")

	raw, err := os.ReadFile(crdPath)
	require.NoError(t, err, "read generated DNSRecord CRD")
	crd := string(raw)

	require.Contains(t, crd, domaindns.FQDNPattern,
		"domaindns.FQDNPattern is not present in the generated CRD — the pre-filter regex drifted from the CRD Pattern; re-run `make helm` or realign the two copies")

	for rt := range domaindns.ValidRecordTypes {
		require.Contains(t, crd, "- "+rt,
			"record type %q accepted by domaindns.ValidRecordTypes is not in the CRD recordType enum — the pre-filter drifted from the CRD Enum", rt)
	}
}
