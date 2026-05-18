package main

import "strings"

// slug normalises a free-form string into a Kubernetes-name-safe value:
// lowercase ASCII letters and digits, all other characters collapsed to '-',
// then trimmed of leading/trailing dashes. Empty results fall back to
// "default" so the caller always gets a non-empty segment.
func slug(s string) string {
	result := make([]byte, 0, len(s))
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
			result = append(result, byte(c))
		case c >= '0' && c <= '9':
			result = append(result, byte(c))
		case c >= 'A' && c <= 'Z':
			result = append(result, byte(c+32))
		default:
			result = append(result, '-')
		}
	}
	out := strings.Trim(string(result), "-")
	if out == "" {
		return "default"
	}
	return out
}
