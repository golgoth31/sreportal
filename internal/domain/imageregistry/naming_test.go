/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package imageregistry

import (
	"regexp"
	"testing"
)

func TestCRName(t *testing.T) {
	t.Parallel()

	rfc1123 := regexp.MustCompile(`^[a-z0-9]+$`)

	tests := []struct {
		name      string
		portal    string
		host      string
		namespace string
	}{
		{name: "simple", portal: tPortalMain, host: "docker.io", namespace: tNsDefault},
		{name: "with-dashes", portal: "team-a", host: "ghcr.io", namespace: "kube-system"},
		{name: "long-host", portal: "p", host: "europe-docker.pkg.dev", namespace: "ns1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := CRName(tc.portal, tc.host, tc.namespace)
			if len(got) != 12 {
				t.Fatalf("CRName length = %d, want 12 (got %q)", len(got), got)
			}
			if !rfc1123.MatchString(got) {
				t.Fatalf("CRName %q must match %s", got, rfc1123)
			}
		})
	}
}

func TestCRName_Stable(t *testing.T) {
	t.Parallel()

	a := CRName(tPortalMain, "docker.io", tNsDefault)
	b := CRName(tPortalMain, "docker.io", tNsDefault)
	if a != b {
		t.Fatalf("CRName not stable: %q vs %q", a, b)
	}
}

func TestCRName_DiffersOnInputs(t *testing.T) {
	t.Parallel()

	base := CRName(tPortalMain, "docker.io", tNsDefault)
	if CRName("other", "docker.io", tNsDefault) == base {
		t.Fatalf("CRName must differ on portal change")
	}
	if CRName(tPortalMain, "ghcr.io", tNsDefault) == base {
		t.Fatalf("CRName must differ on host change")
	}
	if CRName(tPortalMain, "docker.io", "kube-system") == base {
		t.Fatalf("CRName must differ on namespace change")
	}
}
