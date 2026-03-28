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

// MaintenancePhase describes the lifecycle phase of a maintenance window.
// +kubebuilder:validation:Enum=upcoming;in_progress;completed
type MaintenancePhase string

const (
	MaintenancePhaseUpcoming   MaintenancePhase = "upcoming"
	MaintenancePhaseInProgress MaintenancePhase = "in_progress"
	MaintenancePhaseCompleted  MaintenancePhase = "completed"
)

// MaintenanceAffectedStatus describes the status applied to components during a maintenance.
// +kubebuilder:validation:Enum=maintenance;degraded;partial_outage;major_outage
type MaintenanceAffectedStatus string

const (
	MaintenanceAffectedMaintenance MaintenanceAffectedStatus = "maintenance"
	MaintenanceAffectedDegraded    MaintenanceAffectedStatus = "degraded"
	MaintenanceAffectedPartialOut  MaintenanceAffectedStatus = "partial_outage"
	MaintenanceAffectedMajorOutage MaintenanceAffectedStatus = "major_outage"
)

// MaintenanceSpec defines the desired state of Maintenance
type MaintenanceSpec struct {
	// title is the headline displayed on the status page
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Title string `json:"title"`

	// description is a longer explanation (markdown supported in the UI)
	// +optional
	Description string `json:"description,omitempty"`

	// portalRef is the name of the Portal this maintenance is linked to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`

	// components is the list of Component metadata.name values affected by this maintenance
	// +optional
	Components []string `json:"components,omitempty"`

	// scheduledStart is the planned start time of the maintenance window
	// +kubebuilder:validation:Required
	ScheduledStart metav1.Time `json:"scheduledStart"`

	// scheduledEnd is the planned end time of the maintenance window
	// +kubebuilder:validation:Required
	ScheduledEnd metav1.Time `json:"scheduledEnd"`

	// affectedStatus is the status applied to affected components during in_progress phase
	// +kubebuilder:default=maintenance
	AffectedStatus MaintenanceAffectedStatus `json:"affectedStatus"`
}

// MaintenanceStatus defines the observed state of Maintenance.
type MaintenanceStatus struct {
	// phase is the lifecycle phase computed by the controller (upcoming, in_progress, completed)
	// +optional
	Phase MaintenancePhase `json:"phase,omitempty"`

	// conditions represent the current state of the Maintenance resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Start",type=date,JSONPath=`.spec.scheduledStart`
// +kubebuilder:printcolumn:name="End",type=date,JSONPath=`.spec.scheduledEnd`

// Maintenance is the Schema for the maintenances API
type Maintenance struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Maintenance
	// +required
	Spec MaintenanceSpec `json:"spec"`

	// status defines the observed state of Maintenance
	// +optional
	Status MaintenanceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// MaintenanceList contains a list of Maintenance
type MaintenanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Maintenance `json:"items"`
}

// GetConditions returns the status conditions.
func (m *Maintenance) GetConditions() []metav1.Condition { return m.Status.Conditions }

// SetConditions sets the status conditions.
func (m *Maintenance) SetConditions(conditions []metav1.Condition) { m.Status.Conditions = conditions }

func init() {
	SchemeBuilder.Register(&Maintenance{}, &MaintenanceList{})
}
