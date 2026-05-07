/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package imageregistry

import (
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

// ChangeType classifies how a container's runtime image relates to its
// PodSpec template image. See plan §2.1.
type ChangeType string

const (
	// ChangeTypeNone — pod image equals template image.
	ChangeTypeNone ChangeType = "none"
	// ChangeTypeMutated — template declares an image, the pod runs another
	// (typically a webhook rewrote it to a registry mirror).
	ChangeTypeMutated ChangeType = "mutated"
	// ChangeTypeInjected — pod has a container absent from the template
	// (typically a sidecar injected by an admission webhook).
	ChangeTypeInjected ChangeType = "injected"
)

// AllChangeTypes returns the canonical, ordered list of valid change types,
// useful for validation and exhaustive checks.
func AllChangeTypes() []ChangeType {
	return []ChangeType{ChangeTypeNone, ChangeTypeMutated, ChangeTypeInjected}
}

// ContainerObservation is the raw input fed to AggregateForCRs — one entry
// per (workload, container) observed during a cluster scan.
type ContainerObservation struct {
	WorkloadKind      string
	WorkloadName      string
	WorkloadNamespace string
	ContainerName     string
	// TemplateImage is the image declared in the PodSpec template; empty
	// when the container only exists in the running pod (injected).
	TemplateImage string
	// PodImage is the image observed in the running pod; required.
	PodImage string
}

// WorkloadRef identifies a workload/container referencing an Entry. Mirrors
// domain/image.WorkloadRef but carries no `Source` (we don't need to
// distinguish spec vs pod once the ChangeType is known).
type WorkloadRef struct {
	Kind      string
	Namespace string
	Name      string
	Container string
}

// Entry is the aggregated image data for one (originalImage, mutatedImage,
// container-name) tuple inside a (portal, host, namespace) group, ready for
// projection into ImageRegistry.Spec.Images.
type Entry struct {
	// Key — sha256(originalImage|mutatedImage|container)[:16] for use as
	// listMapKey on Spec.Images. Stable across reconciles.
	Key string

	OriginalImage string // empty when ChangeType == injected
	MutatedImage  string

	ChangeType ChangeType

	// Repository, OriginalTag, TagType describe the image targeted by the
	// registry lookup (OriginalImage if non-empty, else MutatedImage).
	Repository  string
	OriginalTag string
	TagType     domainimage.TagType

	// Workloads lists every workload referencing this Entry.
	Workloads []WorkloadRef
}

// Group keys an aggregation by (host, namespace) inside a single portal.
type Group struct {
	Host      string
	Namespace string
}
