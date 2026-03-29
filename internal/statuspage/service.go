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

// Package statuspage provides the write-path service for status page CRs
// (Component, Maintenance, Incident). It manages K8s create/update/delete
// operations with retry-on-conflict.
package statuspage

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

const maxRetries = 5

var (
	ErrNameRequired      = errors.New("name is required")
	ErrPortalRefRequired = errors.New("portal_ref is required")
	ErrGroupRequired     = errors.New("group is required")
	ErrTitleRequired     = errors.New("title is required")
	ErrSeverityRequired  = errors.New("severity is required")
	ErrNotFound          = errors.New("resource not found")
	ErrAlreadyExists     = errors.New("resource already exists")
)

// Service manages status page CRs via the K8s API (write path only).
type Service struct {
	client    client.Client
	namespace string
}

// NewService creates a new status page write Service.
func NewService(c client.Client, namespace string) *Service {
	return &Service{client: c, namespace: namespace}
}

// --- Component ---

// CreateComponentInput holds the fields for creating a Component CR.
type CreateComponentInput struct {
	DisplayName string
	Description string
	Group       string
	Link        string
	PortalRef   string
	Status      sreportalv1alpha1.ComponentStatusValue
}

// CreateComponent creates a new Component CR. Returns (name, error).
func (s *Service) CreateComponent(ctx context.Context, in CreateComponentInput) (string, error) {
	if in.PortalRef == "" {
		return "", ErrPortalRefRequired
	}
	if in.Group == "" {
		return "", ErrGroupRequired
	}

	name := GenerateCRName(in.PortalRef, in.DisplayName)

	nn := types.NamespacedName{Name: name, Namespace: s.namespace}

	var existing sreportalv1alpha1.Component
	if err := s.client.Get(ctx, nn, &existing); err == nil {
		return "", fmt.Errorf("component %q: %w", name, ErrAlreadyExists)
	} else if !apierrors.IsNotFound(err) {
		return "", fmt.Errorf("get component CR: %w", err)
	}

	comp := sreportalv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.namespace},
		Spec: sreportalv1alpha1.ComponentSpec{
			DisplayName: in.DisplayName,
			Description: in.Description,
			Group:       in.Group,
			Link:        in.Link,
			PortalRef:   in.PortalRef,
			Status:      in.Status,
		},
	}
	if err := s.client.Create(ctx, &comp); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return "", fmt.Errorf("component %q: %w", name, ErrAlreadyExists)
		}
		return "", fmt.Errorf("create component CR: %w", err)
	}
	return name, nil
}

// UpdateComponentInput holds the fields for updating an existing Component CR.
type UpdateComponentInput struct {
	Name        string
	DisplayName *string
	Description *string
	Group       *string
	Link        *string
	Status      *sreportalv1alpha1.ComponentStatusValue
}

// UpdateComponent updates an existing Component CR. Returns (name, error).
func (s *Service) UpdateComponent(ctx context.Context, in UpdateComponentInput) (string, error) {
	if in.Name == "" {
		return "", ErrNameRequired
	}

	nn := types.NamespacedName{Name: in.Name, Namespace: s.namespace}

	for attempt := range maxRetries {
		var comp sreportalv1alpha1.Component
		if err := s.client.Get(ctx, nn, &comp); err != nil {
			if apierrors.IsNotFound(err) {
				return "", fmt.Errorf("component %q: %w", in.Name, ErrNotFound)
			}
			return "", fmt.Errorf("get component CR: %w", err)
		}

		if in.DisplayName != nil {
			comp.Spec.DisplayName = *in.DisplayName
		}
		if in.Description != nil {
			comp.Spec.Description = *in.Description
		}
		if in.Group != nil {
			comp.Spec.Group = *in.Group
		}
		if in.Link != nil {
			comp.Spec.Link = *in.Link
		}
		if in.Status != nil {
			comp.Spec.Status = *in.Status
		}

		if err := s.client.Update(ctx, &comp); err != nil {
			if apierrors.IsConflict(err) && attempt < maxRetries-1 {
				continue
			}
			return "", fmt.Errorf("update component CR: %w", err)
		}
		return in.Name, nil
	}

	return "", fmt.Errorf("update component %q: max retries exceeded", in.Name)
}

// DeleteComponent deletes a Component CR.
func (s *Service) DeleteComponent(ctx context.Context, name string) error {
	if name == "" {
		return ErrNameRequired
	}
	comp := &sreportalv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.namespace},
	}
	if err := s.client.Delete(ctx, comp); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("delete component %q: %w", name, ErrNotFound)
		}
		return fmt.Errorf("delete component CR: %w", err)
	}
	return nil
}

// --- Maintenance ---

// CreateMaintenanceInput holds the fields for creating a Maintenance CR.
type CreateMaintenanceInput struct {
	Title          string
	Description    string
	PortalRef      string
	Components     []string
	ScheduledStart metav1.Time
	ScheduledEnd   metav1.Time
	AffectedStatus sreportalv1alpha1.MaintenanceAffectedStatus
}

// CreateMaintenance creates a new Maintenance CR. Returns (name, error).
func (s *Service) CreateMaintenance(ctx context.Context, in CreateMaintenanceInput) (string, error) {
	if in.PortalRef == "" {
		return "", ErrPortalRefRequired
	}
	if in.Title == "" {
		return "", ErrTitleRequired
	}

	name := GenerateCRName(in.PortalRef, in.Title)
	nn := types.NamespacedName{Name: name, Namespace: s.namespace}

	var existing sreportalv1alpha1.Maintenance
	if err := s.client.Get(ctx, nn, &existing); err == nil {
		return "", fmt.Errorf("maintenance %q: %w", name, ErrAlreadyExists)
	} else if !apierrors.IsNotFound(err) {
		return "", fmt.Errorf("get maintenance CR: %w", err)
	}

	affectedStatus := in.AffectedStatus
	if affectedStatus == "" {
		affectedStatus = sreportalv1alpha1.MaintenanceAffectedMaintenance
	}

	maint := sreportalv1alpha1.Maintenance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.namespace},
		Spec: sreportalv1alpha1.MaintenanceSpec{
			Title:          in.Title,
			Description:    in.Description,
			PortalRef:      in.PortalRef,
			Components:     in.Components,
			ScheduledStart: in.ScheduledStart,
			ScheduledEnd:   in.ScheduledEnd,
			AffectedStatus: affectedStatus,
		},
	}
	if err := s.client.Create(ctx, &maint); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return "", fmt.Errorf("maintenance %q: %w", name, ErrAlreadyExists)
		}
		return "", fmt.Errorf("create maintenance CR: %w", err)
	}
	return name, nil
}

// UpdateMaintenanceInput holds the fields for updating an existing Maintenance CR.
type UpdateMaintenanceInput struct {
	Name           string
	Title          *string
	Description    *string
	Components     []string
	ScheduledStart *metav1.Time
	ScheduledEnd   *metav1.Time
	AffectedStatus *sreportalv1alpha1.MaintenanceAffectedStatus
}

// UpdateMaintenance updates an existing Maintenance CR. Returns (name, error).
func (s *Service) UpdateMaintenance(ctx context.Context, in UpdateMaintenanceInput) (string, error) {
	if in.Name == "" {
		return "", ErrNameRequired
	}

	nn := types.NamespacedName{Name: in.Name, Namespace: s.namespace}

	for attempt := range maxRetries {
		var maint sreportalv1alpha1.Maintenance
		if err := s.client.Get(ctx, nn, &maint); err != nil {
			if apierrors.IsNotFound(err) {
				return "", fmt.Errorf("maintenance %q: %w", in.Name, ErrNotFound)
			}
			return "", fmt.Errorf("get maintenance CR: %w", err)
		}

		if in.Title != nil {
			maint.Spec.Title = *in.Title
		}
		if in.Description != nil {
			maint.Spec.Description = *in.Description
		}
		if len(in.Components) > 0 {
			maint.Spec.Components = in.Components
		}
		if in.ScheduledStart != nil {
			maint.Spec.ScheduledStart = *in.ScheduledStart
		}
		if in.ScheduledEnd != nil {
			maint.Spec.ScheduledEnd = *in.ScheduledEnd
		}
		if in.AffectedStatus != nil {
			maint.Spec.AffectedStatus = *in.AffectedStatus
		}

		if err := s.client.Update(ctx, &maint); err != nil {
			if apierrors.IsConflict(err) && attempt < maxRetries-1 {
				continue
			}
			return "", fmt.Errorf("update maintenance CR: %w", err)
		}
		return in.Name, nil
	}

	return "", fmt.Errorf("update maintenance %q: max retries exceeded", in.Name)
}

// DeleteMaintenance deletes a Maintenance CR.
func (s *Service) DeleteMaintenance(ctx context.Context, name string) error {
	if name == "" {
		return ErrNameRequired
	}
	maint := &sreportalv1alpha1.Maintenance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.namespace},
	}
	if err := s.client.Delete(ctx, maint); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("delete maintenance %q: %w", name, ErrNotFound)
		}
		return fmt.Errorf("delete maintenance CR: %w", err)
	}
	return nil
}

// --- Incident ---

// CreateIncidentInput holds the fields for creating an Incident CR.
type CreateIncidentInput struct {
	Title         string
	PortalRef     string
	Components    []string
	Severity      sreportalv1alpha1.IncidentSeverity
	InitialUpdate sreportalv1alpha1.IncidentUpdate
}

// CreateIncident creates a new Incident CR. Returns (name, error).
func (s *Service) CreateIncident(ctx context.Context, in CreateIncidentInput) (string, error) {
	if in.PortalRef == "" {
		return "", ErrPortalRefRequired
	}
	if in.Title == "" {
		return "", ErrTitleRequired
	}
	if in.Severity == "" {
		return "", ErrSeverityRequired
	}

	name := GenerateCRName(in.PortalRef, in.Title)
	nn := types.NamespacedName{Name: name, Namespace: s.namespace}

	var existing sreportalv1alpha1.Incident
	if err := s.client.Get(ctx, nn, &existing); err == nil {
		return "", fmt.Errorf("incident %q: %w", name, ErrAlreadyExists)
	} else if !apierrors.IsNotFound(err) {
		return "", fmt.Errorf("get incident CR: %w", err)
	}

	inc := sreportalv1alpha1.Incident{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.namespace},
		Spec: sreportalv1alpha1.IncidentSpec{
			Title:      in.Title,
			PortalRef:  in.PortalRef,
			Components: in.Components,
			Severity:   in.Severity,
			Updates:    []sreportalv1alpha1.IncidentUpdate{in.InitialUpdate},
		},
	}
	if err := s.client.Create(ctx, &inc); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return "", fmt.Errorf("incident %q: %w", name, ErrAlreadyExists)
		}
		return "", fmt.Errorf("create incident CR: %w", err)
	}
	return name, nil
}

// UpdateIncidentInput holds the fields for updating an existing Incident CR.
type UpdateIncidentInput struct {
	Name       string
	Title      *string
	Components []string
	Severity   *sreportalv1alpha1.IncidentSeverity
	Update     sreportalv1alpha1.IncidentUpdate
}

// UpdateIncident updates an existing Incident CR, appending the update to the timeline.
func (s *Service) UpdateIncident(ctx context.Context, in UpdateIncidentInput) (string, error) {
	if in.Name == "" {
		return "", ErrNameRequired
	}

	nn := types.NamespacedName{Name: in.Name, Namespace: s.namespace}

	for attempt := range maxRetries {
		var inc sreportalv1alpha1.Incident
		if err := s.client.Get(ctx, nn, &inc); err != nil {
			if apierrors.IsNotFound(err) {
				return "", fmt.Errorf("incident %q: %w", in.Name, ErrNotFound)
			}
			return "", fmt.Errorf("get incident CR: %w", err)
		}

		if in.Title != nil {
			inc.Spec.Title = *in.Title
		}
		if len(in.Components) > 0 {
			inc.Spec.Components = in.Components
		}
		if in.Severity != nil {
			inc.Spec.Severity = *in.Severity
		}
		inc.Spec.Updates = append(inc.Spec.Updates, in.Update)

		if err := s.client.Update(ctx, &inc); err != nil {
			if apierrors.IsConflict(err) && attempt < maxRetries-1 {
				continue
			}
			return "", fmt.Errorf("update incident CR: %w", err)
		}
		return in.Name, nil
	}

	return "", fmt.Errorf("update incident %q: max retries exceeded", in.Name)
}

// DeleteIncident deletes an Incident CR.
func (s *Service) DeleteIncident(ctx context.Context, name string) error {
	if name == "" {
		return ErrNameRequired
	}
	inc := &sreportalv1alpha1.Incident{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.namespace},
	}
	if err := s.client.Delete(ctx, inc); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("delete incident %q: %w", name, ErrNotFound)
		}
		return fmt.Errorf("delete incident CR: %w", err)
	}
	return nil
}
