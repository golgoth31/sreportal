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

func mkDS(t *testing.T, mutate func(*sreportalv1alpha1.DeployStatus)) *sreportalv1alpha1.DeployStatus {
	t.Helper()
	ds := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "ds-test", Namespace: tNsDefault},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: tPortalMain,
			Namespace: tNsDefault,
		},
	}
	if mutate != nil {
		mutate(ds)
	}
	return ds
}

func TestDeployStatusValidate_PortalRefRequired(t *testing.T) {
	t.Parallel()
	ds := mkDS(t, func(d *sreportalv1alpha1.DeployStatus) { d.Spec.PortalRef = "" })
	v := &DeployStatusCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), ds); err == nil {
		t.Fatalf("expected error on empty portalRef")
	}
}

func TestDeployStatusValidate_NamespaceRequired(t *testing.T) {
	t.Parallel()
	ds := mkDS(t, func(d *sreportalv1alpha1.DeployStatus) { d.Spec.Namespace = "" })
	v := &DeployStatusCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), ds); err == nil {
		t.Fatalf("expected error on empty namespace")
	}
}

func TestDeployStatusValidate_ServiceKeyRequired(t *testing.T) {
	t.Parallel()
	ds := mkDS(t, func(d *sreportalv1alpha1.DeployStatus) {
		d.Spec.Services = []sreportalv1alpha1.DeployStatusEntry{
			{Key: ""},
		}
	})
	v := &DeployStatusCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), ds)
	if err == nil || !strings.Contains(err.Error(), "key") {
		t.Fatalf("expected error about missing key, got %v", err)
	}
}

func TestDeployStatusValidate_ServiceStateInvalid(t *testing.T) {
	t.Parallel()
	ds := mkDS(t, func(d *sreportalv1alpha1.DeployStatus) {
		d.Spec.Services = []sreportalv1alpha1.DeployStatusEntry{
			{Key: "abc123", State: "unknown"},
		}
	})
	v := &DeployStatusCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), ds)
	if err == nil || !strings.Contains(err.Error(), "state") {
		t.Fatalf("expected error about invalid state, got %v", err)
	}
}

func TestDeployStatusValidate_ServiceStateEmptyAllowed(t *testing.T) {
	t.Parallel()
	ds := mkDS(t, func(d *sreportalv1alpha1.DeployStatus) {
		d.Spec.Services = []sreportalv1alpha1.DeployStatusEntry{
			{Key: "abc123", State: ""},
		}
	})
	v := &DeployStatusCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), ds); err != nil {
		t.Fatalf("empty state should be accepted on spec input entries, got %v", err)
	}
}

func TestDeployStatusValidate_ValidNoServices(t *testing.T) {
	t.Parallel()
	ds := mkDS(t, nil)
	v := &DeployStatusCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), ds); err != nil {
		t.Fatalf("expected valid object with no services, got %v", err)
	}
}

func TestDeployStatusValidate_ValidWithServices(t *testing.T) {
	t.Parallel()
	ds := mkDS(t, func(d *sreportalv1alpha1.DeployStatus) {
		d.Spec.Services = []sreportalv1alpha1.DeployStatusEntry{
			{Key: "svc1", State: "ok"},
			{Key: "svc2", State: "behind"},
			{Key: "svc3", State: "unresolved"},
			{Key: "svc4", State: "error"},
			{Key: "svc5", State: ""},
		}
	})
	v := &DeployStatusCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), ds); err != nil {
		t.Fatalf("expected valid object, got %v", err)
	}
	// Update path also valid.
	if _, err := v.ValidateUpdate(context.Background(), ds, ds); err != nil {
		t.Fatalf("ValidateUpdate: %v", err)
	}
	// Delete is no-op.
	if _, err := v.ValidateDelete(context.Background(), ds); err != nil {
		t.Fatalf("ValidateDelete: %v", err)
	}
}
