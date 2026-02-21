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

import "strings"

// GroupsAnnotationKey is the protocol annotation used to assign an endpoint to one or
// more groups. Multiple groups are expressed as a comma-separated list.
// This annotation takes the highest priority over all other grouping rules.
const GroupsAnnotationKey = "sreportal.io/groups"

// GroupMappingStrategy resolves the group name(s) for an endpoint based on its labels
// and namespace. Rules are evaluated in priority order:
//
//  1. sreportal.io/groups annotation — comma-separated, yields multiple groups
//  2. Configured LabelKey label — yields a single group
//  3. ByNamespace mapping — yields a single group
//  4. DefaultGroup fallback — yields a single group
//
// GroupMappingStrategy is a pure value type with no external dependencies,
// safe for concurrent use.
type GroupMappingStrategy struct {
	// DefaultGroup is the group name for endpoints that match no other rule.
	DefaultGroup string
	// LabelKey is the endpoint label key whose value is used as the group name.
	LabelKey string
	// ByNamespace maps a Kubernetes namespace to a group name.
	ByNamespace map[string]string
}

// Resolve returns the group names for an endpoint identified by its labels and
// namespace. It always returns at least one element.
func (s GroupMappingStrategy) Resolve(labels map[string]string, namespace string) []string {
	// 1. sreportal.io/groups annotation — highest priority, comma-separated.
	if val := labels[GroupsAnnotationKey]; val != "" {
		parts := strings.Split(val, ",")
		groups := make([]string, 0, len(parts))
		for _, p := range parts {
			if g := strings.TrimSpace(p); g != "" {
				groups = append(groups, g)
			}
		}
		if len(groups) > 0 {
			return groups
		}
	}

	// 2. Configured label key.
	if s.LabelKey != "" {
		if val := labels[s.LabelKey]; val != "" {
			return []string{val}
		}
	}

	// 3. Namespace mapping.
	if namespace != "" && len(s.ByNamespace) > 0 {
		if group, ok := s.ByNamespace[namespace]; ok && group != "" {
			return []string{group}
		}
	}

	// 4. Default group.
	if s.DefaultGroup != "" {
		return []string{s.DefaultGroup}
	}

	return []string{"Services"}
}
