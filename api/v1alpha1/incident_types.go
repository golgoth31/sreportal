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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IncidentSeverity describes how severe an incident is.
// +kubebuilder:validation:Enum=critical;major;minor
type IncidentSeverity string

const (
	IncidentSeverityCritical IncidentSeverity = "critical"
	IncidentSeverityMajor    IncidentSeverity = "major"
	IncidentSeverityMinor    IncidentSeverity = "minor"
)

// IncidentPhase describes the current resolution stage of an incident.
// +kubebuilder:validation:Enum=investigating;identified;monitoring;resolved
type IncidentPhase string

const (
	IncidentPhaseInvestigating IncidentPhase = "investigating"
	IncidentPhaseIdentified    IncidentPhase = "identified"
	IncidentPhaseMonitoring    IncidentPhase = "monitoring"
	IncidentPhaseResolved      IncidentPhase = "resolved"
)

// IncidentUpdate represents a single timeline entry in the incident lifecycle.
type IncidentUpdate struct {
	// timestamp is the time of this update
	// +kubebuilder:validation:Required
	Timestamp metav1.Time `json:"timestamp"`

	// phase is the incident phase at the time of this update
	// +kubebuilder:validation:Required
	Phase IncidentPhase `json:"phase"`

	// message is a human-readable description of the update
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Message string `json:"message"`
}

// IncidentSpec defines the desired state of Incident
type IncidentSpec struct {
	// title is the headline of the incident
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Title string `json:"title"`

	// portalRef is the name of the Portal this incident is linked to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`

	// components is the list of Component metadata.name values affected
	// +optional
	Components []string `json:"components,omitempty"`

	// severity indicates the impact level of the incident
	// +kubebuilder:validation:Required
	Severity IncidentSeverity `json:"severity"`

	// updates is the chronological timeline of the incident, appended via kubectl edit/patch
	// +optional
	Updates []IncidentUpdate `json:"updates,omitempty"`
}

// IncidentStatus defines the observed state of Incident.
type IncidentStatus struct {
	// currentPhase is the phase from the most recent update (by timestamp)
	// +optional
	CurrentPhase IncidentPhase `json:"currentPhase,omitempty"`

	// startedAt is the timestamp of the first update
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// resolvedAt is the timestamp of the first update with phase=resolved
	// +optional
	ResolvedAt *metav1.Time `json:"resolvedAt,omitempty"`

	// durationMinutes is the incident duration in minutes (computed when resolved)
	// +optional
	DurationMinutes int `json:"durationMinutes,omitempty"`

	// conditions represent the current state of the Incident resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Severity",type=string,JSONPath=`.spec.severity`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.currentPhase`
// +kubebuilder:printcolumn:name="Started",type=date,JSONPath=`.status.startedAt`
// +kubebuilder:printcolumn:name="Resolved",type=date,JSONPath=`.status.resolvedAt`

// Incident is the Schema for the incidents API
type Incident struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Incident
	// +required
	Spec IncidentSpec `json:"spec"`

	// status defines the observed state of Incident
	// +optional
	Status IncidentStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// IncidentList contains a list of Incident
type IncidentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Incident `json:"items"`
}

// GetConditions returns the status conditions.
func (i *Incident) GetConditions() []metav1.Condition { return i.Status.Conditions }

// SetConditions sets the status conditions.
func (i *Incident) SetConditions(conditions []metav1.Condition) { i.Status.Conditions = conditions }

func init() {
	SchemeBuilder.Register(&Incident{}, &IncidentList{})
}
