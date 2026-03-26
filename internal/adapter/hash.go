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

package adapter

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// hashExcludedLabels lists label keys that are excluded from hash computation
// because they are unstable across controller restarts or enrichment runs.
var hashExcludedLabels = map[string]struct{}{
	endpoint.ResourceLabelKey:       {},
	"sreportal.io/origin-kind":      {},
	"sreportal.io/origin-namespace": {},
	"sreportal.io/origin-name":      {},
}

// EndpointsHash computes a stable SHA-256 hex digest of the source-provided
// endpoint data. Only DNSName, RecordType, Targets, and stable Labels are
// included. TTL, LastSeen, and resource/origin labels are excluded so that
// the hash remains stable across ticks when the source data hasn't changed.
//
// The result is order-independent: endpoints and targets are sorted before hashing.
func EndpointsHash(endpoints []*endpoint.Endpoint) string {
	lines := make([]string, 0, len(endpoints))

	for _, ep := range endpoints {
		lines = append(lines, endpointLine(ep.DNSName, ep.RecordType, ep.Targets, ep.Labels))
	}

	return hashLines(lines)
}

// EndpointStatusHash computes the same stable SHA-256 hex digest from
// EndpointStatus objects. It produces the same hash as EndpointsHash for
// equivalent data, allowing comparison between source-collected endpoints
// and persisted status.
func EndpointStatusHash(statuses []sreportalv1alpha1.EndpointStatus) string {
	lines := make([]string, 0, len(statuses))

	for _, s := range statuses {
		lines = append(lines, endpointLine(s.DNSName, s.RecordType, s.Targets, s.Labels))
	}

	return hashLines(lines)
}

// endpointLine builds a canonical string representation of a single endpoint
// for hashing purposes.
func endpointLine(dnsName, recordType string, targets []string, labels map[string]string) string {
	sortedTargets := make([]string, len(targets))
	copy(sortedTargets, targets)
	sort.Strings(sortedTargets)

	sortedLabels := stableLabels(labels)

	return fmt.Sprintf("%s|%s|%s|%s", dnsName, recordType,
		strings.Join(sortedTargets, ","), sortedLabels)
}

// stableLabels returns a deterministic string representation of labels,
// excluding unstable keys (resource refs, origin refs).
func stableLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	keys := make([]string, 0, len(labels))
	for k := range labels {
		if _, excluded := hashExcludedLabels[k]; excluded {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+labels[k])
	}

	return strings.Join(pairs, ";")
}

// hashLines sorts the lines for order independence, then computes SHA-256.
func hashLines(lines []string) string {
	sort.Strings(lines)

	h := sha256.New()
	for _, line := range lines {
		h.Write([]byte(line))
		h.Write([]byte{'\n'})
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}
