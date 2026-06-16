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

// Package chain contains Chain-of-Responsibility handlers for the DNSRecord controller.
package chain

import v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"

// ChainData holds shared state between handlers during a DNSRecord reconciliation.
type ChainData struct {
	// ResourceKey is the namespaced key used to address the read store entry.
	ResourceKey string
	// GroupMapping holds the DNS CR's group mapping configuration.
	GroupMapping *v1alpha2.GroupMappingSpec
	// DisableDNSCheck mirrors DNS CR's spec.reconciliation.disableDNSCheck.
	DisableDNSCheck bool
	// OwnerDNSName is the name of the owning DNS CR (from controller ownerRef).
	// Used by the project store handler to annotate the read store so that
	// per-DNS conflict reporting can scope events to a specific DNS owner.
	OwnerDNSName string
}
