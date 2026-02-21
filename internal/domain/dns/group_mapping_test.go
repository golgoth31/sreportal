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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/domain/dns"
)

func TestGroupMappingStrategy_Resolve(t *testing.T) {
	fullStrategy := dns.GroupMappingStrategy{
		DefaultGroup: "Default",
		LabelKey:     "app.io/group",
		ByNamespace:  map[string]string{"prod": "Production", "staging": "Staging"},
	}

	cases := []struct {
		name      string
		strategy  dns.GroupMappingStrategy
		labels    map[string]string
		namespace string
		want      []string
	}{
		{
			name:      "groups annotation yields multiple groups",
			strategy:  fullStrategy,
			labels:    map[string]string{dns.GroupsAnnotationKey: "APIs, Applications"},
			namespace: "prod",
			want:      []string{"APIs", "Applications"},
		},
		{
			name:      "groups annotation trims whitespace",
			strategy:  fullStrategy,
			labels:    map[string]string{dns.GroupsAnnotationKey: " A , B , C "},
			namespace: "",
			want:      []string{"A", "B", "C"},
		},
		{
			name:      "groups annotation takes priority over label key",
			strategy:  fullStrategy,
			labels:    map[string]string{dns.GroupsAnnotationKey: "FromAnnotation", "app.io/group": "FromLabel"},
			namespace: "prod",
			want:      []string{"FromAnnotation"},
		},
		{
			name:      "groups annotation takes priority over namespace mapping",
			strategy:  fullStrategy,
			labels:    map[string]string{dns.GroupsAnnotationKey: "FromAnnotation"},
			namespace: "prod",
			want:      []string{"FromAnnotation"},
		},
		{
			name:      "label key used when no groups annotation",
			strategy:  fullStrategy,
			labels:    map[string]string{"app.io/group": "FromLabel"},
			namespace: "prod",
			want:      []string{"FromLabel"},
		},
		{
			name:      "label key takes priority over namespace mapping",
			strategy:  fullStrategy,
			labels:    map[string]string{"app.io/group": "FromLabel"},
			namespace: "prod",
			want:      []string{"FromLabel"},
		},
		{
			name:      "namespace mapping used when no annotation or label",
			strategy:  fullStrategy,
			labels:    map[string]string{},
			namespace: "prod",
			want:      []string{"Production"},
		},
		{
			name:      "staging namespace maps correctly",
			strategy:  fullStrategy,
			labels:    map[string]string{},
			namespace: "staging",
			want:      []string{"Staging"},
		},
		{
			name:      "unknown namespace falls back to default group",
			strategy:  fullStrategy,
			labels:    map[string]string{},
			namespace: "unknown",
			want:      []string{"Default"},
		},
		{
			name:      "empty namespace falls back to default group",
			strategy:  fullStrategy,
			labels:    map[string]string{},
			namespace: "",
			want:      []string{"Default"},
		},
		{
			name:      "no rules match uses default group",
			strategy:  dns.GroupMappingStrategy{DefaultGroup: "Fallback"},
			labels:    map[string]string{},
			namespace: "",
			want:      []string{"Fallback"},
		},
		{
			name:      "empty strategy returns hardcoded Services",
			strategy:  dns.GroupMappingStrategy{},
			labels:    map[string]string{},
			namespace: "",
			want:      []string{"Services"},
		},
		{
			name:      "nil label map does not panic",
			strategy:  fullStrategy,
			labels:    nil,
			namespace: "",
			want:      []string{"Default"},
		},
		{
			name:      "groups annotation with only whitespace entries is ignored",
			strategy:  fullStrategy,
			labels:    map[string]string{dns.GroupsAnnotationKey: " , , "},
			namespace: "",
			want:      []string{"Default"},
		},
		{
			name:      "groups annotation single value without comma",
			strategy:  fullStrategy,
			labels:    map[string]string{dns.GroupsAnnotationKey: "SingleGroup"},
			namespace: "",
			want:      []string{"SingleGroup"},
		},
		{
			name:      "label key not configured falls through to namespace",
			strategy:  dns.GroupMappingStrategy{DefaultGroup: "Default", ByNamespace: map[string]string{"prod": "Prod"}},
			labels:    map[string]string{"app.io/group": "SomeValue"},
			namespace: "prod",
			want:      []string{"Prod"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.strategy.Resolve(tc.labels, tc.namespace)
			require.Equal(t, tc.want, got)
		})
	}
}
