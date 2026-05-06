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
			MutatedImage:  "mirror.io/library/nginx:1.25.0",
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

func TestImageRegistryValidate_Valid(t *testing.T) {
	t.Parallel()
	ir := mkIR(t, func(r *sreportalv1alpha1.ImageRegistry) {
		r.Spec.Images = []sreportalv1alpha1.ImageRegistrySpecEntry{
			{Key: "k1", OriginalImage: tImgNginxDocker, MutatedImage: tImgNginxDocker, ChangeType: tChangeTypeNone},
			{Key: "k2", OriginalImage: "docker.io/library/redis:7", MutatedImage: "mirror.io/library/redis:7", ChangeType: tChangeTypeMut},
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
