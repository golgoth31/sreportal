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

// PortalSpec defines the desired state of Portal
type PortalSpec struct {
	// title is the display title for this portal
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Title string `json:"title"`

	// main marks this portal as the default portal for unmatched FQDNs
	// +optional
	Main bool `json:"main,omitempty"`

	// subPath is the URL subpath for this portal (defaults to metadata.name)
	// +optional
	SubPath string `json:"subPath,omitempty"`

	// remote configures this portal to fetch data from a remote SRE Portal instance.
	// When set, the operator will fetch DNS information from the remote portal
	// instead of collecting data from the local cluster.
	// This field cannot be set when main is true.
	// +optional
	Remote *RemotePortalSpec `json:"remote,omitempty"`
}

// RemotePortalSpec defines the configuration for fetching data from a remote portal.
type RemotePortalSpec struct {
	// url is the base URL of the remote SRE Portal instance.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://.*`
	URL string `json:"url"`

	// portal is the name of the portal to target on the remote instance.
	// If not set, the main portal of the remote instance will be used.
	// +optional
	Portal string `json:"portal,omitempty"`

	// tls configures TLS settings for connecting to the remote portal.
	// If not set, the default system TLS configuration is used.
	// +optional
	TLS *RemoteTLSConfig `json:"tls,omitempty"`
}

// RemoteTLSConfig defines the TLS configuration for connecting to a remote portal.
type RemoteTLSConfig struct {
	// insecureSkipVerify disables TLS certificate verification when connecting
	// to the remote portal. Use with caution: this makes the connection
	// susceptible to man-in-the-middle attacks.
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

	// caSecretRef references a Secret containing a custom CA certificate bundle.
	// The Secret must contain the key "ca.crt".
	// +optional
	CASecretRef *SecretRef `json:"caSecretRef,omitempty"`

	// certSecretRef references a Secret containing a client certificate and key for mTLS.
	// The Secret must contain the keys "tls.crt" and "tls.key".
	// +optional
	CertSecretRef *SecretRef `json:"certSecretRef,omitempty"`
}

// SecretRef is a reference to a Kubernetes Secret in the same namespace.
type SecretRef struct {
	// name is the name of the Secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// PortalStatus defines the observed state of Portal.
type PortalStatus struct {
	// ready indicates if the portal is fully configured
	// +optional
	Ready bool `json:"ready,omitempty"`

	// conditions represent the current state of the Portal resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// remoteSync contains the status of synchronization with a remote portal.
	// This is only populated when spec.remote is set.
	// +optional
	RemoteSync *RemoteSyncStatus `json:"remoteSync,omitempty"`
}

// RemoteSyncStatus contains status information about remote portal synchronization.
type RemoteSyncStatus struct {
	// lastSyncTime is the timestamp of the last successful synchronization.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// lastSyncError contains the error message from the last failed synchronization attempt.
	// Empty if the last sync was successful.
	// +optional
	LastSyncError string `json:"lastSyncError,omitempty"`

	// remoteTitle is the title of the remote portal as fetched from the remote server.
	// +optional
	RemoteTitle string `json:"remoteTitle,omitempty"`

	// fqdnCount is the number of FQDNs fetched from the remote portal.
	// +optional
	FQDNCount int `json:"fqdnCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=portals,scope=Namespaced
// +kubebuilder:printcolumn:name="Title",type=string,JSONPath=`.spec.title`
// +kubebuilder:printcolumn:name="Main",type=boolean,JSONPath=`.spec.main`
// +kubebuilder:printcolumn:name="Remote URL",type=string,JSONPath=`.spec.remote.url`,priority=1
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Portal is the Schema for the portals API
type Portal struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Portal
	// +required
	Spec PortalSpec `json:"spec"`

	// status defines the observed state of Portal
	// +optional
	Status PortalStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PortalList contains a list of Portal
type PortalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Portal `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Portal{}, &PortalList{})
}
