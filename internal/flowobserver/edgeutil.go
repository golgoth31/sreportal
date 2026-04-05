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
