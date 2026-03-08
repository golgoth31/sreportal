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

// AlertmanagerSpec defines the desired state of Alertmanager
type AlertmanagerSpec struct {
	// portalRef is the name of the Portal this Alertmanager resource is linked to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`

	// url contains the Alertmanager API endpoints
	// +kubebuilder:validation:Required
	URL AlertmanagerURL `json:"url"`

	// IsRemote indicates that the corresponding portal is remote and the operator should fetch alerts from the remote portal instead of local Alertmanager API.
	// This field is used to determine how to fetch alerts.
	// +optional
	IsRemote bool `json:"isRemote,omitempty"`
}

// AlertmanagerURL holds the local and remote Alertmanager API URLs
type AlertmanagerURL struct {
	// local is the URL used by the controller to fetch active alerts (e.g. http://alertmanager.monitoring:9093)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://.*`
	Local string `json:"local"`

	// remote is an optional externally-reachable URL for dashboard links
	// +optional
	// +kubebuilder:validation:Pattern=`^https?://.*`
	Remote string `json:"remote,omitempty"`
}

// AlertmanagerStatus defines the observed state of Alertmanager.
type AlertmanagerStatus struct {
	// activeAlerts is the list of currently firing alerts retrieved from the Alertmanager API
	// +optional
	ActiveAlerts []AlertStatus `json:"activeAlerts,omitempty"`

	// conditions represent the current state of the Alertmanager resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// lastReconcileTime is the timestamp of the last reconciliation
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// remoteAlertmanagerURL is the externally-reachable Alertmanager URL fetched from
	// a remote portal. Only populated when spec.isRemote is true.
	// +optional
	RemoteAlertmanagerURL string `json:"remoteAlertmanagerURL,omitempty"`
}

// AlertStatus represents a single active alert from Alertmanager
type AlertStatus struct {
	// fingerprint is the unique identifier of the alert
	Fingerprint string `json:"fingerprint"`

	// labels are the identifying key-value pairs of the alert
	Labels map[string]string `json:"labels"`

	// annotations are additional informational key-value pairs
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// state is the alert state (active, suppressed, unprocessed)
	// +kubebuilder:validation:Enum=active;suppressed;unprocessed
	State string `json:"state"`

	// startsAt is when the alert started firing
	StartsAt metav1.Time `json:"startsAt"`

	// endsAt is when the alert is expected to resolve
	// +optional
	EndsAt *metav1.Time `json:"endsAt,omitempty"`

	// updatedAt is the last time the alert was updated
	UpdatedAt metav1.Time `json:"updatedAt"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Active Alerts",type=integer,JSONPath=`.status.activeAlerts`
// +kubebuilder:printcolumn:name="Last Reconcile",type=date,JSONPath=`.status.lastReconcileTime`

// Alertmanager is the Schema for the alertmanagers API
type Alertmanager struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Alertmanager
	// +required
	Spec AlertmanagerSpec `json:"spec"`

	// status defines the observed state of Alertmanager
	// +optional
	Status AlertmanagerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AlertmanagerList contains a list of Alertmanager
type AlertmanagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Alertmanager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Alertmanager{}, &AlertmanagerList{})
}
