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

package flowobserver

import "testing"

func TestParseNodeID(t *testing.T) {
	tests := []struct {
		id                                string
		wantType, wantNamespace, wantName string
	}{
		{"service:core:api-server", "service", "core", "api-server"},
		{"database:data:postgres-main", "database", "data", "postgres-main"},
		{"external:default:api.stripe.com", "external", "default", "api.stripe.com"},
		{"cron:jobs:cleanup-cron", "cron", "jobs", "cleanup-cron"},
		// Edge case: name with colons
		{"service:ns:name:with:colons", "service", "ns", "name:with:colons"},
		// Invalid formats
		{"", "", "", ""},
		{"service", "", "", ""},
		{"service:ns", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			gotType, gotNs, gotName := ParseNodeID(tt.id)
			if gotType != tt.wantType || gotNs != tt.wantNamespace || gotName != tt.wantName {
				t.Errorf("ParseNodeID(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tt.id, gotType, gotNs, gotName, tt.wantType, tt.wantNamespace, tt.wantName)
			}
		})
	}
}
