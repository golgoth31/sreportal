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
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

// TestFQDNValidationMatchesCRD guards the duplication between the domain
// validators (domaindns.FQDNPattern / domaindns.ValidRecordTypes) and the
// DNSRecord CRD constraints. The DNS controller pre-filters entries with the
// domain copies; if they ever drift from the CRD the whole "skip one bad entry
// instead of aborting the reconcile" fix silently breaks — entries would pass
// the pre-filter yet be rejected at admission (too loose), or be dropped though
// the CRD would accept them (too strict).
//
// It parses the controller-gen source of truth (config/crd/bases) as typed
// schema — not a substring scan — and asserts exact equality in BOTH directions
// (domain == CRD). The helm-templated manifest is generated from the same
// markers by helmify in the same `make helm`; it is a Go template (not valid
// standalone YAML), so it only gets a cheap sanity check that the pattern is
// present, catching a gross bases/helm divergence.
func TestFQDNValidationMatchesCRD(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// internal/domain/dns -> repo root.
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	basePath := filepath.Join(root, "config", "crd", "bases", "sreportal.io_dnsrecords.yaml")
	helmPath := filepath.Join(root, "helm", "templates", "dnsrecord-crd.yaml")

	fqdn, recordType := entrySchema(t, basePath)

	require.Equal(t, domaindns.FQDNPattern, fqdn.Pattern,
		"CRD entries.fqdn pattern drifted from domaindns.FQDNPattern (re-run `make helm` or realign)")

	crdTypes := map[string]struct{}{}
	for _, e := range recordType.Enum {
		var v string
		require.NoError(t, json.Unmarshal(e.Raw, &v))
		crdTypes[v] = struct{}{}
	}
	require.Equal(t, domaindns.ValidRecordTypes, crdTypes,
		"CRD entries.recordType enum drifted from domaindns.ValidRecordTypes (must be equal in both directions)")

	helm, err := os.ReadFile(helmPath)
	require.NoError(t, err, "read helm-templated CRD")
	require.Contains(t, string(helm), domaindns.FQDNPattern,
		"helm-templated CRD lost the FQDN pattern present in config/crd/bases (re-run `make helm`)")
}

// entrySchema returns the fqdn and recordType schema nodes of DNSRecordEntry
// from the storage version of the generated CRD, or fails the test.
func entrySchema(t *testing.T, crdPath string) (fqdn, recordType apiextv1.JSONSchemaProps) {
	t.Helper()
	raw, err := os.ReadFile(crdPath)
	require.NoError(t, err, "read generated DNSRecord CRD")

	var crd apiextv1.CustomResourceDefinition
	require.NoError(t, yaml.Unmarshal(raw, &crd), "unmarshal CRD %s", crdPath)

	var schema *apiextv1.JSONSchemaProps
	for i := range crd.Spec.Versions {
		v := crd.Spec.Versions[i]
		if v.Storage && v.Schema != nil {
			schema = v.Schema.OpenAPIV3Schema
			break
		}
	}
	require.NotNil(t, schema, "no storage version schema in %s", crdPath)

	spec, ok := schema.Properties["spec"]
	require.True(t, ok, "spec not found in %s", crdPath)
	entries, ok := spec.Properties["entries"]
	require.True(t, ok, "spec.entries not found in %s", crdPath)
	require.NotNil(t, entries.Items, "spec.entries.items not found in %s", crdPath)
	require.NotNil(t, entries.Items.Schema, "spec.entries.items.schema not found in %s", crdPath)
	props := entries.Items.Schema.Properties

	fqdn, ok = props["fqdn"]
	require.True(t, ok, "spec.entries.items.fqdn not found in %s", crdPath)
	recordType, ok = props["recordType"]
	require.True(t, ok, "spec.entries.items.recordType not found in %s", crdPath)
	return fqdn, recordType
}
