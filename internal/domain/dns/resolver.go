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

package dns

import (
	"context"
	"sort"
	"strings"
)

// SyncStatus represents the DNS resolution status of an FQDN.
type SyncStatus string

const (
	// SyncStatusSync indicates the FQDN resolves to the expected type and targets.
	SyncStatusSync SyncStatus = "sync"
	// SyncStatusNotAvailable indicates the FQDN does not exist in DNS.
	SyncStatusNotAvailable SyncStatus = "notavailable"
	// SyncStatusNotSync indicates the FQDN exists but resolves to different targets.
	SyncStatusNotSync SyncStatus = "notsync"
)

// Resolver abstracts DNS lookups for testability.
type Resolver interface {
	LookupHost(ctx context.Context, fqdn string) ([]string, error)
	LookupCNAME(ctx context.Context, fqdn string) (string, error)
}

// CheckResult holds the outcome of a DNS resolution check.
type CheckResult struct {
	Status          SyncStatus
	ResolvedTargets []string
}

// CheckFQDN verifies whether an FQDN resolves correctly in DNS.
//   - A/AAAA: LookupHost and compare sorted IPs with sorted expected targets.
//   - CNAME: LookupCNAME and compare with the first expected target.
//   - Empty recordType (manual entry): LookupHost to check existence only.
func CheckFQDN(ctx context.Context, r Resolver, fqdn, recordType string, targets []string) *CheckResult {
	switch strings.ToUpper(recordType) {
	case "A", "AAAA":
		return checkHostRecord(ctx, r, fqdn, targets)
	case "CNAME":
		return checkCNAMERecord(ctx, r, fqdn, targets)
	default:
		return checkExistence(ctx, r, fqdn)
	}
}

func checkHostRecord(ctx context.Context, r Resolver, fqdn string, expectedTargets []string) *CheckResult {
	addrs, err := r.LookupHost(ctx, fqdn)
	if err != nil {
		return &CheckResult{Status: SyncStatusNotAvailable}
	}

	if targetsMatch(expectedTargets, addrs) {
		return &CheckResult{Status: SyncStatusSync, ResolvedTargets: addrs}
	}

	return &CheckResult{Status: SyncStatusNotSync, ResolvedTargets: addrs}
}

func checkCNAMERecord(ctx context.Context, r Resolver, fqdn string, expectedTargets []string) *CheckResult {
	cname, err := r.LookupCNAME(ctx, fqdn)
	if err != nil {
		return &CheckResult{Status: SyncStatusNotAvailable}
	}

	// net.LookupCNAME always returns a fully-qualified name with a trailing dot
	// (e.g. "real.example.com."), while external-dns stores targets without one
	// (e.g. "real.example.com"). Strip trailing dots from both sides before comparing.
	if len(expectedTargets) > 0 &&
		strings.EqualFold(strings.TrimSuffix(cname, "."), strings.TrimSuffix(expectedTargets[0], ".")) {
		return &CheckResult{Status: SyncStatusSync, ResolvedTargets: []string{cname}}
	}

	return &CheckResult{Status: SyncStatusNotSync, ResolvedTargets: []string{cname}}
}

func checkExistence(ctx context.Context, r Resolver, fqdn string) *CheckResult {
	addrs, err := r.LookupHost(ctx, fqdn)
	if err != nil || len(addrs) == 0 {
		return &CheckResult{Status: SyncStatusNotAvailable}
	}

	return &CheckResult{Status: SyncStatusSync, ResolvedTargets: addrs}
}

// targetsMatch compares two string slices after sorting and normalizing.
func targetsMatch(expected, actual []string) bool {
	if len(expected) != len(actual) {
		return false
	}

	e := sortedCopy(expected)
	a := sortedCopy(actual)

	for i := range e {
		if !strings.EqualFold(e[i], a[i]) {
			return false
		}
	}

	return true
}

func sortedCopy(s []string) []string {
	cp := make([]string, len(s))
	copy(cp, s)
	sort.Strings(cp)
	return cp
}
