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

// Package dns contains pure domain types for DNS management.
// This package has no external dependencies and contains only business logic.
package dns

import (
	"sort"
	"strings"
	"time"
)

// Source represents the origin of an FQDN
type Source string

const (
	// SourceManual indicates a manually configured FQDN
	SourceManual Source = "manual"
	// SourceExternalDNS indicates an FQDN discovered from external-dns
	SourceExternalDNS Source = "external-dns"
)

// FQDN represents a fully qualified domain name with metadata
type FQDN struct {
	// Name is the fully qualified domain name
	Name string

	// Source indicates where this FQDN came from
	Source Source

	// Description is an optional human-readable description
	Description string

	// RecordType is the DNS record type (A, AAAA, CNAME, etc.)
	RecordType string

	// Targets is the list of target addresses for this FQDN
	Targets []string

	// LastSeen is the timestamp when this FQDN was last observed
	LastSeen time.Time
}

// NewFQDN creates a new FQDN with the given name and source
func NewFQDN(name string, source Source) *FQDN {
	return &FQDN{
		Name:     strings.ToLower(strings.TrimSpace(name)),
		Source:   source,
		LastSeen: time.Now(),
	}
}

// WithDescription sets the description and returns the FQDN
func (f *FQDN) WithDescription(desc string) *FQDN {
	f.Description = desc
	return f
}

// WithRecordType sets the record type and returns the FQDN
func (f *FQDN) WithRecordType(recordType string) *FQDN {
	f.RecordType = recordType
	return f
}

// WithTargets sets the targets and returns the FQDN
func (f *FQDN) WithTargets(targets []string) *FQDN {
	f.Targets = targets
	return f
}

// FQDNCollection is a collection of FQDNs with deduplication logic
type FQDNCollection struct {
	fqdns map[string]*FQDN
}

// NewFQDNCollection creates a new empty FQDNCollection
func NewFQDNCollection() *FQDNCollection {
	return &FQDNCollection{
		fqdns: make(map[string]*FQDN),
	}
}

// Add adds an FQDN to the collection.
// If an FQDN with the same name already exists, manual entries take precedence.
func (c *FQDNCollection) Add(fqdn *FQDN) {
	existing, exists := c.fqdns[fqdn.Name]
	if !exists {
		c.fqdns[fqdn.Name] = fqdn
		return
	}

	// Manual entries take precedence over external-dns
	if fqdn.Source == SourceManual && existing.Source == SourceExternalDNS {
		// Keep existing targets and record type from external-dns
		fqdn.Targets = existing.Targets
		fqdn.RecordType = existing.RecordType
		c.fqdns[fqdn.Name] = fqdn
		return
	}

	// Update LastSeen if the new entry is more recent
	if fqdn.LastSeen.After(existing.LastSeen) {
		existing.LastSeen = fqdn.LastSeen
	}
}

// Get returns an FQDN by name, or nil if not found
func (c *FQDNCollection) Get(name string) *FQDN {
	return c.fqdns[strings.ToLower(strings.TrimSpace(name))]
}

// All returns all FQDNs in the collection, sorted by name
func (c *FQDNCollection) All() []*FQDN {
	result := make([]*FQDN, 0, len(c.fqdns))
	for _, fqdn := range c.fqdns {
		result = append(result, fqdn)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Count returns the number of FQDNs in the collection
func (c *FQDNCollection) Count() int {
	return len(c.fqdns)
}

// FilterBySource returns FQDNs filtered by source
func (c *FQDNCollection) FilterBySource(source Source) []*FQDN {
	result := make([]*FQDN, 0)
	for _, fqdn := range c.fqdns {
		if fqdn.Source == source {
			result = append(result, fqdn)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// GroupBySource returns FQDNs grouped by source
func (c *FQDNCollection) GroupBySource() map[Source][]*FQDN {
	groups := make(map[Source][]*FQDN)
	for _, fqdn := range c.fqdns {
		groups[fqdn.Source] = append(groups[fqdn.Source], fqdn)
	}
	// Sort each group
	for source := range groups {
		sort.Slice(groups[source], func(i, j int) bool {
			return groups[source][i].Name < groups[source][j].Name
		})
	}
	return groups
}
