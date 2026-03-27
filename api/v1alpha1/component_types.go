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

// ComponentStatusValue describes the operational status that a user can declare on a component.
// +kubebuilder:validation:Enum=operational;degraded;partial_outage;major_outage;unknown
type ComponentStatusValue string

const (
	ComponentStatusOperational ComponentStatusValue = "operational"
	ComponentStatusDegraded    ComponentStatusValue = "degraded"
	ComponentStatusPartialOut  ComponentStatusValue = "partial_outage"
	ComponentStatusMajorOutage ComponentStatusValue = "major_outage"
	ComponentStatusUnknown     ComponentStatusValue = "unknown"
)

// ComputedComponentStatus extends ComponentStatusValue with controller-only values (e.g. "maintenance").
// It is used in status.computedStatus and may contain values not settable by users.
type ComputedComponentStatus string

const (
	ComputedStatusOperational  ComputedComponentStatus = "operational"
	ComputedStatusDegraded     ComputedComponentStatus = "degraded"
	ComputedStatusPartialOut   ComputedComponentStatus = "partial_outage"
	ComputedStatusMajorOutage  ComputedComponentStatus = "major_outage"
	ComputedStatusUnknown      ComputedComponentStatus = "unknown"
	ComputedStatusMaintenance  ComputedComponentStatus = "maintenance"
)

// ComponentSpec defines the desired state of Component
type ComponentSpec struct {
	// displayName is the human-readable name shown on the status page
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`

	// description is a short text displayed below the component name
	// +optional
	Description string `json:"description,omitempty"`

	// group is a logical grouping for the status page (e.g. "Infrastructure", "Applications")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Group string `json:"group"`

	// link is an optional external URL (e.g. GCP console, Grafana dashboard)
	// +optional
	// +kubebuilder:validation:Pattern=`^https?://.*`
	Link string `json:"link,omitempty"`

	// portalRef is the name of the Portal this component is linked to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`

	// status is the manually declared operational status
	// +kubebuilder:default=unknown
	Status ComponentStatusValue `json:"status"`
}

// ComponentStatus defines the observed state of Component.
type ComponentStatus struct {
	// computedStatus is the effective status calculated by the controller.
	// If a maintenance is in progress on this component, it is overridden to "maintenance".
	// Otherwise it reflects spec.status.
	// +optional
	ComputedStatus ComputedComponentStatus `json:"computedStatus,omitempty"`

	// activeIncidents is the number of active (non-resolved) incidents linked to this component
	// +optional
	ActiveIncidents int `json:"activeIncidents,omitempty"`

	// lastStatusChange is the timestamp of the last computedStatus transition
	// +optional
	LastStatusChange *metav1.Time `json:"lastStatusChange,omitempty"`

	// conditions represent the current state of the Component resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Group",type=string,JSONPath=`.spec.group`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.spec.status`
// +kubebuilder:printcolumn:name="Computed",type=string,JSONPath=`.status.computedStatus`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Component is the Schema for the components API
type Component struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Component
	// +required
	Spec ComponentSpec `json:"spec"`

	// status defines the observed state of Component
	// +optional
	Status ComponentStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ComponentList contains a list of Component
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Component `json:"items"`
}

// GetConditions returns the status conditions.
func (c *Component) GetConditions() []metav1.Condition { return c.Status.Conditions }

// SetConditions sets the status conditions.
func (c *Component) SetConditions(conditions []metav1.Condition) { c.Status.Conditions = conditions }

func init() {
	SchemeBuilder.Register(&Component{}, &ComponentList{})
}
