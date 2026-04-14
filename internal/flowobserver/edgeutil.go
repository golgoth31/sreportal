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

import "strings"

// ParseNodeID extracts type, namespace, and name from a node ID like "service:core:api-server".
// Returns empty strings if the format is invalid.
func ParseNodeID(id string) (nodeType, namespace, name string) {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) != 3 {
		return "", "", ""
	}

	return parts[0], parts[1], parts[2]
}
