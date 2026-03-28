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

// ComponentInput holds the fields for creating or updating a Component CR.
type ComponentInput struct {
	Name        string
	DisplayName string
	Description string
	Group       string
	Link        string
	PortalRef   string
	Status      sreportalv1alpha1.ComponentStatusValue
}

// UpsertComponent creates or updates a Component CR. Returns (name, created, error).
func (s *Service) UpsertComponent(ctx context.Context, in ComponentInput) (string, bool, error) {
	if in.Name == "" {
		return "", false, ErrNameRequired
	}
	if in.PortalRef == "" {
		return "", false, ErrPortalRefRequired
	}
	if in.Group == "" {
		return "", false, ErrGroupRequired
	}

	nn := types.NamespacedName{Name: in.Name, Namespace: s.namespace}

	for attempt := range maxRetries {
		var comp sreportalv1alpha1.Component
		err := s.client.Get(ctx, nn, &comp)

		if apierrors.IsNotFound(err) {
			comp = sreportalv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{Name: in.Name, Namespace: s.namespace},
				Spec: sreportalv1alpha1.ComponentSpec{
					DisplayName: in.DisplayName,
					Description: in.Description,
					Group:       in.Group,
					Link:        in.Link,
					PortalRef:   in.PortalRef,
					Status:      in.Status,
				},
			}
			if createErr := s.client.Create(ctx, &comp); createErr != nil {
				if apierrors.IsAlreadyExists(createErr) {
					continue
				}
				return "", false, fmt.Errorf("create component CR: %w", createErr)
			}
			return in.Name, true, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("get component CR: %w", err)
		}

		// Update existing
		comp.Spec.DisplayName = in.DisplayName
		comp.Spec.Description = in.Description
		comp.Spec.Group = in.Group
		comp.Spec.Link = in.Link
		comp.Spec.PortalRef = in.PortalRef
		comp.Spec.Status = in.Status
		if updateErr := s.client.Update(ctx, &comp); updateErr != nil {
			if apierrors.IsConflict(updateErr) && attempt < maxRetries-1 {
				continue
			}
			return "", false, fmt.Errorf("update component CR: %w", updateErr)
		}
		return in.Name, false, nil
	}

	return "", false, fmt.Errorf("upsert component %q: max retries exceeded", in.Name)
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

// MaintenanceInput holds the fields for creating or updating a Maintenance CR.
type MaintenanceInput struct {
	Name           string
	Title          string
	Description    string
	PortalRef      string
	Components     []string
	ScheduledStart metav1.Time
	ScheduledEnd   metav1.Time
	AffectedStatus sreportalv1alpha1.MaintenanceAffectedStatus
}

// UpsertMaintenance creates or updates a Maintenance CR. Returns (name, created, error).
func (s *Service) UpsertMaintenance(ctx context.Context, in MaintenanceInput) (string, bool, error) {
	if in.Name == "" {
		return "", false, ErrNameRequired
	}
	if in.PortalRef == "" {
		return "", false, ErrPortalRefRequired
	}
	if in.Title == "" {
		return "", false, ErrTitleRequired
	}

	nn := types.NamespacedName{Name: in.Name, Namespace: s.namespace}

	for attempt := range maxRetries {
		var maint sreportalv1alpha1.Maintenance
		err := s.client.Get(ctx, nn, &maint)

		if apierrors.IsNotFound(err) {
			affectedStatus := in.AffectedStatus
			if affectedStatus == "" {
				affectedStatus = sreportalv1alpha1.MaintenanceAffectedMaintenance
			}
			maint = sreportalv1alpha1.Maintenance{
				ObjectMeta: metav1.ObjectMeta{Name: in.Name, Namespace: s.namespace},
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
			if createErr := s.client.Create(ctx, &maint); createErr != nil {
				if apierrors.IsAlreadyExists(createErr) {
					continue
				}
				return "", false, fmt.Errorf("create maintenance CR: %w", createErr)
			}
			return in.Name, true, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("get maintenance CR: %w", err)
		}

		// Update existing
		maint.Spec.Title = in.Title
		maint.Spec.Description = in.Description
		maint.Spec.PortalRef = in.PortalRef
		maint.Spec.Components = in.Components
		maint.Spec.ScheduledStart = in.ScheduledStart
		maint.Spec.ScheduledEnd = in.ScheduledEnd
		if in.AffectedStatus != "" {
			maint.Spec.AffectedStatus = in.AffectedStatus
		}
		if updateErr := s.client.Update(ctx, &maint); updateErr != nil {
			if apierrors.IsConflict(updateErr) && attempt < maxRetries-1 {
				continue
			}
			return "", false, fmt.Errorf("update maintenance CR: %w", updateErr)
		}
		return in.Name, false, nil
	}

	return "", false, fmt.Errorf("upsert maintenance %q: max retries exceeded", in.Name)
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

// IncidentInput holds the fields for creating or updating an Incident CR.
type IncidentInput struct {
	Name       string
	Title      string
	PortalRef  string
	Components []string
	Severity   sreportalv1alpha1.IncidentSeverity
	Updates    []sreportalv1alpha1.IncidentUpdate
}

// UpsertIncident creates or updates an Incident CR. Returns (name, created, error).
func (s *Service) UpsertIncident(ctx context.Context, in IncidentInput) (string, bool, error) {
	if in.Name == "" {
		return "", false, ErrNameRequired
	}
	if in.PortalRef == "" {
		return "", false, ErrPortalRefRequired
	}
	if in.Title == "" {
		return "", false, ErrTitleRequired
	}
	if in.Severity == "" {
		return "", false, ErrSeverityRequired
	}

	nn := types.NamespacedName{Name: in.Name, Namespace: s.namespace}

	for attempt := range maxRetries {
		var inc sreportalv1alpha1.Incident
		err := s.client.Get(ctx, nn, &inc)

		if apierrors.IsNotFound(err) {
			inc = sreportalv1alpha1.Incident{
				ObjectMeta: metav1.ObjectMeta{Name: in.Name, Namespace: s.namespace},
				Spec: sreportalv1alpha1.IncidentSpec{
					Title:      in.Title,
					PortalRef:  in.PortalRef,
					Components: in.Components,
					Severity:   in.Severity,
					Updates:    in.Updates,
				},
			}
			if createErr := s.client.Create(ctx, &inc); createErr != nil {
				if apierrors.IsAlreadyExists(createErr) {
					continue
				}
				return "", false, fmt.Errorf("create incident CR: %w", createErr)
			}
			return in.Name, true, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("get incident CR: %w", err)
		}

		// Update existing
		inc.Spec.Title = in.Title
		inc.Spec.PortalRef = in.PortalRef
		inc.Spec.Components = in.Components
		inc.Spec.Severity = in.Severity
		inc.Spec.Updates = in.Updates
		if updateErr := s.client.Update(ctx, &inc); updateErr != nil {
			if apierrors.IsConflict(updateErr) && attempt < maxRetries-1 {
				continue
			}
			return "", false, fmt.Errorf("update incident CR: %w", updateErr)
		}
		return in.Name, false, nil
	}

	return "", false, fmt.Errorf("upsert incident %q: max retries exceeded", in.Name)
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
