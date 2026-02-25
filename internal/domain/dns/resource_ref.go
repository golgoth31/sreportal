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
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidResourceRef is returned when a raw resource label cannot be parsed
// into a valid ResourceRef. The expected format is "kind/namespace/name".
var ErrInvalidResourceRef = errors.New("invalid resource ref: expected kind/namespace/name format")

// ResourceRef is an immutable value object that identifies a Kubernetes resource
// by its kind, namespace, and name.
//
// It is parsed from the external-dns resource label
// (external-dns.alpha.kubernetes.io/resource) whose value follows the pattern
// "kind/namespace/name" — e.g. "service/default/my-svc".
//
// Two ResourceRefs with identical fields are considered equal (value semantics).
type ResourceRef struct {
	kind      string
	namespace string
	name      string
}

// ParseResourceRef parses a raw resource label produced by external-dns.
// The expected format is "kind/namespace/name" with no empty segments.
// Returns ErrInvalidResourceRef if the format is invalid.
func ParseResourceRef(raw string) (ResourceRef, error) {
	parts := strings.SplitN(raw, "/", 3)
	if len(parts) != 3 {
		return ResourceRef{}, fmt.Errorf("%w: %q", ErrInvalidResourceRef, raw)
	}

	kind := strings.TrimSpace(parts[0])
	ns := strings.TrimSpace(parts[1])
	name := strings.TrimSpace(parts[2])

	if kind == "" || ns == "" || name == "" {
		return ResourceRef{}, fmt.Errorf("%w: %q", ErrInvalidResourceRef, raw)
	}

	return ResourceRef{kind: kind, namespace: ns, name: name}, nil
}

// Kind returns the Kubernetes resource kind (e.g. "service", "ingress").
func (r ResourceRef) Kind() string { return r.kind }

// Namespace returns the Kubernetes namespace of the resource.
func (r ResourceRef) Namespace() string { return r.namespace }

// Name returns the Kubernetes resource name.
func (r ResourceRef) Name() string { return r.name }

// IsZero reports whether the ResourceRef is the zero value (unparsed).
func (r ResourceRef) IsZero() bool { return r.kind == "" }
