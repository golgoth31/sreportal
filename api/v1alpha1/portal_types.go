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

	// features controls which features are enabled for this portal.
	// All features default to true when not specified.
	// +optional
	Features *PortalFeatures `json:"features,omitempty"`

	// auth configures authentication for Connect write RPCs targeting this portal.
	// Local portals with spec.auth unset inherit the main portal's auth (spec.main: true).
	// Remote portals must set spec.auth explicitly (no inheritance from main).
	// +optional
	Auth *PortalAuthSpec `json:"auth,omitempty"`
}

// PortalFeatures controls which features are enabled for a portal.
// All features default to true when not specified.
type PortalFeatures struct {
	// dns enables DNS discovery (controllers, gRPC, MCP, web page) for this portal.
	// +optional
	// +kubebuilder:default=true
	DNS *bool `json:"dns,omitempty"`

	// releases enables the releases page for this portal.
	// +optional
	// +kubebuilder:default=true
	Releases *bool `json:"releases,omitempty"`

	// networkPolicy enables network policy visualization for this portal.
	// +optional
	// +kubebuilder:default=true
	NetworkPolicy *bool `json:"networkPolicy,omitempty"`

	// alerts enables alertmanager integration for this portal.
	// +optional
	// +kubebuilder:default=true
	Alerts *bool `json:"alerts,omitempty"`

	// statusPage enables the status page (components, incidents, maintenances) for this portal.
	// +optional
	// +kubebuilder:default=true
	StatusPage *bool `json:"statusPage,omitempty"`

	// auth overrides per feature (optional). When set for a feature, replaces portal-level
	// and main inheritance for that feature's write RPCs only.
	// +optional
	Auth *PortalFeatureAuthOverrides `json:"auth,omitempty"`
}

// IsDNSEnabled returns true if DNS feature is enabled (nil-safe, defaults to true).
func (f *PortalFeatures) IsDNSEnabled() bool {
	return f == nil || f.DNS == nil || *f.DNS
}

// IsReleasesEnabled returns true if releases feature is enabled (nil-safe, defaults to true).
func (f *PortalFeatures) IsReleasesEnabled() bool {
	return f == nil || f.Releases == nil || *f.Releases
}

// IsNetworkPolicyEnabled returns true if network policy feature is enabled (nil-safe, defaults to true).
func (f *PortalFeatures) IsNetworkPolicyEnabled() bool {
	return f == nil || f.NetworkPolicy == nil || *f.NetworkPolicy
}

// IsAlertsEnabled returns true if alerts feature is enabled (nil-safe, defaults to true).
func (f *PortalFeatures) IsAlertsEnabled() bool {
	return f == nil || f.Alerts == nil || *f.Alerts
}

// IsStatusPageEnabled returns true if status page feature is enabled (nil-safe, defaults to true).
func (f *PortalFeatures) IsStatusPageEnabled() bool {
	return f == nil || f.StatusPage == nil || *f.StatusPage
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

// PortalAuthSpec configures authentication for Connect write endpoints (API key and/or JWT).
// Multiple methods can be enabled; the server accepts a request if any method succeeds.
type PortalAuthSpec struct {
	// apiKey configures header-based API key authentication.
	// The secret value is read from the referenced Secret (key defaults to "api-key").
	// +optional
	APIKey *PortalAPIKeyAuth `json:"apiKey,omitempty"`

	// jwt configures JWT Bearer token authentication (JWKS).
	// +optional
	JWT *PortalJWTAuthSpec `json:"jwt,omitempty"`
}

// Enabled returns true if at least one authentication method is enabled.
func (a *PortalAuthSpec) Enabled() bool {
	if a == nil {
		return false
	}
	if a.APIKey != nil && a.APIKey.Enabled {
		return true
	}
	if a.JWT != nil && a.JWT.Enabled {
		return true
	}
	return false
}

// PortalAPIKeyAuth configures API key authentication via an HTTP header and a Kubernetes Secret.
type PortalAPIKeyAuth struct {
	// enabled controls whether API key authentication is active.
	Enabled bool `json:"enabled"`

	// headerName is the HTTP header to check (default: "X-API-Key").
	// +optional
	HeaderName string `json:"headerName,omitempty"`

	// secretRef references a Secret in the portal's namespace containing the API key.
	// Required when enabled is true.
	SecretRef SecretRef `json:"secretRef"`

	// secretKey is the key within the Secret's data (default: "api-key").
	// +optional
	SecretKey string `json:"secretKey,omitempty"`
}

// PortalJWTAuthSpec configures JWT Bearer token authentication.
type PortalJWTAuthSpec struct {
	// enabled controls whether JWT authentication is active.
	Enabled bool `json:"enabled"`

	// issuers lists trusted JWT issuers (JWKS). Required when enabled is true.
	Issuers []PortalJWTIssuerSpec `json:"issuers"`
}

// PortalJWTIssuerSpec configures a single JWT issuer.
type PortalJWTIssuerSpec struct {
	// name is a display name for this issuer.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// issuerURL is the expected JWT "iss" claim.
	// +kubebuilder:validation:Required
	IssuerURL string `json:"issuerURL"`

	// audience is the expected "aud" claim when non-empty.
	// +optional
	Audience string `json:"audience,omitempty"`

	// jwksURL is the JWKS endpoint URL for this issuer.
	// +kubebuilder:validation:Required
	JWKSURL string `json:"jwksURL"`

	// requiredClaims lists additional claim name/value pairs that must match.
	// +optional
	RequiredClaims map[string]string `json:"requiredClaims,omitempty"`
}

// PortalFeatureAuthOverrides holds optional auth overrides per portal feature.
type PortalFeatureAuthOverrides struct {
	// dns overrides auth for DNS-related write RPCs (reserved for future use).
	// +optional
	DNS *PortalAuthSpec `json:"dns,omitempty"`

	// releases overrides auth for release write RPCs (e.g. AddRelease).
	// +optional
	Releases *PortalAuthSpec `json:"releases,omitempty"`

	// networkPolicy overrides auth for network policy write RPCs (reserved for future use).
	// +optional
	NetworkPolicy *PortalAuthSpec `json:"networkPolicy,omitempty"`

	// alerts overrides auth for alert-related write RPCs (reserved for future use).
	// +optional
	Alerts *PortalAuthSpec `json:"alerts,omitempty"`

	// statusPage overrides auth for status page write RPCs (components, incidents, maintenances).
	// +optional
	StatusPage *PortalAuthSpec `json:"statusPage,omitempty"`
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

	// features contains the feature flags reported by the remote portal.
	// Used to compute effective features for remote portals (local AND remote).
	// +optional
	Features *PortalFeaturesStatus `json:"features,omitempty"`
}

// PortalFeaturesStatus contains the observed feature flags from a remote portal.
// Unlike PortalFeatures (spec), these are explicit booleans with no nil-defaults-to-true semantics.
type PortalFeaturesStatus struct {
	// dns indicates whether the remote portal has DNS discovery enabled.
	DNS bool `json:"dns"`

	// releases indicates whether the remote portal has releases enabled.
	Releases bool `json:"releases"`

	// networkPolicy indicates whether the remote portal has network policy visualization enabled.
	NetworkPolicy bool `json:"networkPolicy"`

	// alerts indicates whether the remote portal has alertmanager integration enabled.
	Alerts bool `json:"alerts"`

	// statusPage indicates whether the remote portal has the status page enabled.
	StatusPage bool `json:"statusPage"`
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
