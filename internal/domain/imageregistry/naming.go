/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Package imageregistry holds the domain logic for the ImageRegistry CRD:
// per (portal, host, namespace) aggregation of observed images, semver
// resolution and stable CR naming. All packages in this directory are pure —
// no infrastructure dependencies, no Kubernetes client.
package imageregistry

import (
	"crypto/sha256"
	"encoding/hex"
)

// crNameLen is the byte length of the truncated sha256 used as ImageRegistry
// CR name. 12 hex chars = 48 bits of entropy ≈ 1.8e-12 collision probability
// for 1000 entries (cf. plan §16). Always ≤ 63 chars (RFC 1123 namespace
// limit) and matches `[a-z0-9]+`.
const crNameLen = 12

// CRName returns a deterministic, RFC 1123-safe CR name for the
// (portal, host, namespace) tuple by hashing them with sha256 and
// truncating to 12 lowercase hex chars.
//
// The pipe separator avoids collisions between e.g. ("a|b", "c", "d") and
// ("a", "b|c", "d") — both inputs produce distinct hashes.
func CRName(portal, host, namespace string) string {
	sum := sha256.Sum256([]byte(portal + "|" + host + "|" + namespace))
	return hex.EncodeToString(sum[:])[:crNameLen]
}
