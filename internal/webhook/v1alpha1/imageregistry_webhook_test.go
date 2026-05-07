/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

func mkIR(t *testing.T, mutate func(*sreportalv1alpha1.ImageRegistry)) *sreportalv1alpha1.ImageRegistry {
	t.Helper()
	ir := &sreportalv1alpha1.ImageRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "abc123def456", Namespace: "default"},
		Spec: sreportalv1alpha1.ImageRegistrySpec{
			Host:      "docker.io",
			PortalRef: "main",
			Namespace: "default",
		},
	}
	if mutate != nil {
		mutate(ir)
	}
	return ir
}

func TestImageRegistryValidate_HostRequired(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) { r.Spec.Host = "" })
	v := &ImageRegistryCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), ir); err == nil {
		t.Fatalf("expected error on empty host")
	}
}

func TestImageRegistryValidate_HostInvalid(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) { r.Spec.Host = "not a host" })
	v := &ImageRegistryCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), ir); err == nil {
		t.Fatalf("expected error on invalid host")
	}
}

func TestImageRegistryValidate_PortalRefRequired(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) { r.Spec.PortalRef = "" })
	v := &ImageRegistryCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), ir); err == nil {
		t.Fatalf("expected error on empty portalRef")
	}
}

func TestImageRegistryValidate_NamespaceRequired(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) { r.Spec.Namespace = "" })
	v := &ImageRegistryCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), ir); err == nil {
		t.Fatalf("expected error on empty namespace")
	}
}

func TestImageRegistryValidate_NoneRequiresOriginal(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) {
		r.Spec.Images = []sreportalv1alpha1.ImageRegistrySpecEntry{{
			Key:           "k",
			MutatedImage:  tImgNginxDocker,
			OriginalImage: "",
			ChangeType:    tChangeTypeNone,
		}}
	})
	v := &ImageRegistryCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), ir)
	if err == nil || !strings.Contains(err.Error(), "originalImage") {
		t.Fatalf("expected error about missing originalImage, got %v", err)
	}
}

func TestImageRegistryValidate_MutatedRequiresOriginal(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) {
		r.Spec.Images = []sreportalv1alpha1.ImageRegistrySpecEntry{{
			Key:           "k",
			MutatedImage:  tImgNginxDocker,
			OriginalImage: "",
			ChangeType:    tChangeTypeMut,
		}}
	})
	v := &ImageRegistryCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), ir)
	if err == nil || !strings.Contains(err.Error(), "originalImage") {
		t.Fatalf("expected error about missing originalImage, got %v", err)
	}
}

func TestImageRegistryValidate_InjectedForbidsOriginal(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) {
		r.Spec.Images = []sreportalv1alpha1.ImageRegistrySpecEntry{{
			Key:           "k",
			MutatedImage:  "docker.io/istio/proxy:1.20.0",
			OriginalImage: "should-not-be-here",
			ChangeType:    tChangeTypeInj,
		}}
	})
	v := &ImageRegistryCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), ir)
	if err == nil || !strings.Contains(err.Error(), "forbids originalImage") {
		t.Fatalf("expected error about forbidden originalImage, got %v", err)
	}
}

func TestImageRegistryValidate_BadChangeType(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) {
		r.Spec.Images = []sreportalv1alpha1.ImageRegistrySpecEntry{{
			Key:          "k",
			MutatedImage: tImgNginxDocker,
			ChangeType:   "weird",
		}}
	})
	v := &ImageRegistryCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), ir)
	if err == nil {
		t.Fatalf("expected error on bad changeType")
	}
}

func TestImageRegistryValidate_MutatedImageRequired(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) {
		r.Spec.Images = []sreportalv1alpha1.ImageRegistrySpecEntry{{
			Key:           "k",
			MutatedImage:  "",
			OriginalImage: "x",
			ChangeType:    tChangeTypeNone,
		}}
	})
	v := &ImageRegistryCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), ir)
	if err == nil || !strings.Contains(err.Error(), "mutatedImage") {
		t.Fatalf("expected error about mutatedImage, got %v", err)
	}
}

func TestImageRegistryValidate_KeyRequired(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) {
		r.Spec.Images = []sreportalv1alpha1.ImageRegistrySpecEntry{{
			Key:           "",
			MutatedImage:  "x",
			OriginalImage: "x",
			ChangeType:    tChangeTypeNone,
		}}
	})
	v := &ImageRegistryCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), ir)
	if err == nil || !strings.Contains(err.Error(), "key") {
		t.Fatalf("expected error about key, got %v", err)
	}
}

func TestImageRegistryValidate_HostDenylist(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		host string
	}{
		{"loopback v4", "127.0.0.1"},
		{"loopback v6", "::1"},
		{"localhost name", "localhost"},
		{"link-local IMDS", "169.254.169.254"},
		{"rfc1918 10/8", "10.0.0.1"},
		{"rfc1918 172.16/12", "172.16.0.5"},
		{"rfc1918 192.168/16", "192.168.1.1"},
		{"unspecified", "0.0.0.0"},
		{"in-cluster svc", "registry.kube-system.svc.cluster.local"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) { r.Spec.Host = c.host })
			v := &ImageRegistryCustomValidator{}
			_, err := v.ValidateCreate(context.Background(), ir)
			if err == nil {
				t.Fatalf("expected error for forbidden host %q", c.host)
			}
		})
	}
}

// TestImageRegistryValidate_HostCoherence_MutatedDivergesAllowed verifies that
// for ChangeType=mutated, MutatedImage may live in a different registry than
// spec.host. Rationale: the lookup target (used by the controller to query
// versions) is OriginalImage. MutatedImage is the rewritten reference produced
// by an admission controller (pull-through cache, registry mirror) and is
// allowed to diverge.
func TestImageRegistryValidate_HostCoherence_MutatedDivergesAllowed(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) {
		r.Spec.Host = tHostGHCR
		r.Spec.Images = []sreportalv1alpha1.ImageRegistrySpecEntry{{
			Key:           "k",
			OriginalImage: "ghcr.io/myorg/myapp:1.0.0",
			MutatedImage:  tImgNginxDocker, // docker.io — mirrored/rewritten
			ChangeType:    tChangeTypeMut,
		}}
	})
	v := &ImageRegistryCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), ir); err != nil {
		t.Fatalf("expected MutatedImage host divergence to be accepted for changeType=mutated, got %v", err)
	}
}

func TestImageRegistryValidate_HostCoherence_InjectedMutatedMismatch(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) {
		r.Spec.Host = tHostGHCR
		r.Spec.Images = []sreportalv1alpha1.ImageRegistrySpecEntry{{
			Key:          "k",
			MutatedImage: tImgNginxDocker, // docker.io — must match spec.host for injected
			ChangeType:   tChangeTypeInj,
		}}
	})
	v := &ImageRegistryCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), ir)
	if err == nil || !strings.Contains(err.Error(), "does not match spec.host") {
		t.Fatalf("expected host mismatch error for injected mutatedImage, got %v", err)
	}
}

func TestImageRegistryValidate_HostCoherence_OriginalMismatch(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) {
		r.Spec.Host = tHostGHCR
		r.Spec.Images = []sreportalv1alpha1.ImageRegistrySpecEntry{{
			Key:           "k",
			OriginalImage: tImgNginxDocker,
			MutatedImage:  "ghcr.io/myorg/myapp:1.0.0",
			ChangeType:    tChangeTypeMut,
		}}
	})
	v := &ImageRegistryCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), ir)
	if err == nil || !strings.Contains(err.Error(), "does not match spec.host") {
		t.Fatalf("expected host mismatch error, got %v", err)
	}
}

func TestImageRegistryValidate_DockerHubAliasesEqual(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) {
		r.Spec.Host = "index.docker.io"
		r.Spec.Images = []sreportalv1alpha1.ImageRegistrySpecEntry{{
			Key:           "k",
			OriginalImage: tImgNginxDocker, // docker.io/...
			MutatedImage:  tImgNginxDocker,
			ChangeType:    tChangeTypeNone,
		}}
	})
	v := &ImageRegistryCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), ir); err != nil {
		t.Fatalf("docker.io and index.docker.io should be aliases, got %v", err)
	}
}

func TestImageRegistryValidate_Valid(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) {
		r.Spec.Images = []sreportalv1alpha1.ImageRegistrySpecEntry{
			{Key: "k1", OriginalImage: tImgNginxDocker, MutatedImage: tImgNginxDocker, ChangeType: tChangeTypeNone},
			{Key: "k2", OriginalImage: "docker.io/library/redis:7", MutatedImage: "docker.io/library/redis:7", ChangeType: tChangeTypeMut},
			{Key: "k3", MutatedImage: "docker.io/istio/proxy:1.20", ChangeType: tChangeTypeInj},
		}
	})
	v := &ImageRegistryCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), ir)
	if err != nil {
		t.Fatalf("expected valid object, got %v", err)
	}
	// Update path also valid.
	if _, err := v.ValidateUpdate(context.Background(), ir, ir); err != nil {
		t.Fatalf("ValidateUpdate: %v", err)
	}
	// Delete is no-op.
	if _, err := v.ValidateDelete(context.Background(), ir); err != nil {
		t.Fatalf("ValidateDelete: %v", err)
	}
}
