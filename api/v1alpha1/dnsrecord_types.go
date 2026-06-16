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
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

const annotationV1Alpha2DNSRecordSpec = "sreportal.io/v1alpha2-dnsrecord-spec"

// preservedDNSRecordSpec holds v1alpha2-only DNSRecordSpec fields that have no
// v1alpha1 representation. It is JSON-encoded into
// annotationV1Alpha2DNSRecordSpec on ConvertFrom (hub → spoke) and restored on
// ConvertTo (spoke → hub).
type preservedDNSRecordSpec struct {
	Origin  v1alpha2.DNSRecordOrigin  `json:"origin"`
	Entries []v1alpha2.DNSRecordEntry `json:"entries,omitempty"`
}

// DNSRecordSpec defines the desired state of DNSRecord
type DNSRecordSpec struct {
	// sourceType indicates the external-dns source type that provides this record.
	// Empty when the v1alpha2 hub object has origin=manual (no source — entries
	// live in the v1alpha2-only annotation). The conversion would otherwise
	// produce a v1alpha1 object that fails its own validation.
	// +optional
	// +kubebuilder:validation:Enum="";service;ingress;dnsendpoint;istio-gateway;istio-virtualservice;gateway-httproute;gateway-grpcroute;gateway-tlsroute;gateway-tcproute;gateway-udproute
	SourceType string `json:"sourceType,omitempty"`

	// portalRef is the name of the Portal this record belongs to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`
}

// DNSRecordStatus defines the observed state of DNSRecord
type DNSRecordStatus struct {
	// endpoints contains the DNS endpoints discovered from this source
	// +optional
	Endpoints []EndpointStatus `json:"endpoints,omitempty"`

	// endpointsHash is a SHA-256 digest of the source-provided endpoint data
	// (DNSName, RecordType, Targets, Labels). It is used by the SourceReconciler
	// to skip status updates when endpoints have not changed between ticks.
	// +optional
	EndpointsHash string `json:"endpointsHash,omitempty"`

	// lastReconcileTime is the timestamp of the last reconciliation
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// conditions represent the current state of the DNSRecord resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// EndpointStatus represents a single DNS endpoint discovered from external-dns
type EndpointStatus struct {
	// dnsName is the fully qualified domain name
	// +kubebuilder:validation:Required
	DNSName string `json:"dnsName"`

	// recordType is the DNS record type (A, AAAA, CNAME, TXT, etc.)
	// +optional
	RecordType string `json:"recordType,omitempty"`

	// targets is the list of target addresses for this endpoint
	// +optional
	Targets []string `json:"targets,omitempty"`

	// ttl is the DNS record TTL in seconds
	// +optional
	TTL int64 `json:"ttl,omitempty"`

	// labels contains the endpoint labels from external-dns
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// syncStatus indicates whether the endpoint is correctly resolved in DNS.
	// sync: the FQDN resolves to the expected type and targets.
	// notavailable: the FQDN does not exist in DNS.
	// notsync: the FQDN exists but resolves to different targets or type.
	// +kubebuilder:validation:Enum=sync;notavailable;notsync;""
	// +optional
	SyncStatus string `json:"syncStatus,omitempty"`

	// lastSeen is the timestamp when this endpoint was last observed
	// +kubebuilder:validation:Required
	LastSeen metav1.Time `json:"lastSeen"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=dnsrecords,scope=Namespaced,shortName=dnsrec
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.sourceType`
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Endpoints",type=integer,JSONPath=`.status.endpoints`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DNSRecord is the Schema for the dnsrecords API.
// It represents DNS endpoints discovered from a specific external-dns source.
type DNSRecord struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DNSRecord
	// +required
	Spec DNSRecordSpec `json:"spec"`

	// status defines the observed state of DNSRecord
	// +optional
	Status DNSRecordStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DNSRecordList contains a list of DNSRecord
type DNSRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DNSRecord `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNSRecord{}, &DNSRecordList{})
}

// ConvertTo converts this DNSRecord (v1alpha1) to the Hub version (v1alpha2).
// Fresh v1alpha1 records are auto-discovered (origin=auto). Records that
// originated from v1alpha2 carry annotationV1Alpha2DNSRecordSpec and have
// their Origin/Entries restored from it.
func (src *DNSRecord) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.DNSRecord)
	// Deep copy ObjectMeta so subsequent mutations (annotation strip) do not
	// affect the source — the apiserver may pass cached objects in.
	dst.ObjectMeta = *src.ObjectMeta.DeepCopy()
	dst.Spec.Origin = v1alpha2.DNSRecordOriginAuto
	dst.Spec.PortalRef = src.Spec.PortalRef
	dst.Spec.SourceType = v1alpha2.SourceType(src.Spec.SourceType)

	if raw, ok := src.Annotations[annotationV1Alpha2DNSRecordSpec]; ok && raw != "" {
		var p preservedDNSRecordSpec
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			return fmt.Errorf("unmarshal v1alpha2 DNSRecordSpec annotation on %s/%s: %w", src.Namespace, src.Name, err)
		}
		dst.Spec.Origin = p.Origin
		dst.Spec.Entries = p.Entries
		if p.Origin == v1alpha2.DNSRecordOriginManual {
			dst.Spec.SourceType = "" // mutex with entries
		}
		delete(dst.Annotations, annotationV1Alpha2DNSRecordSpec)
	}

	for _, e := range src.Status.Endpoints {
		dst.Status.Endpoints = append(dst.Status.Endpoints, v1alpha2.EndpointStatus{
			DNSName:    e.DNSName,
			RecordType: e.RecordType,
			Targets:    e.Targets,
			TTL:        e.TTL,
			Labels:     e.Labels,
			SyncStatus: v1alpha2.SyncStatus(e.SyncStatus),
			LastSeen:   e.LastSeen,
		})
	}
	dst.Status.EndpointsHash = src.Status.EndpointsHash
	dst.Status.LastReconcileTime = src.Status.LastReconcileTime
	dst.Status.Conditions = src.Status.Conditions
	return nil
}

// ConvertFrom converts from the Hub version (v1alpha2) to this DNSRecord (v1alpha1).
// v1alpha1 cannot represent manual entries or the origin discriminator natively,
// so they are stashed in annotationV1Alpha2DNSRecordSpec for lossless round-trip.
func (dst *DNSRecord) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.DNSRecord)
	// Deep copy ObjectMeta so subsequent mutations (annotation insert) do not
	// affect the source — the apiserver may pass cached objects in.
	dst.ObjectMeta = *src.ObjectMeta.DeepCopy()
	dst.Spec.PortalRef = src.Spec.PortalRef
	dst.Spec.SourceType = string(src.Spec.SourceType)

	preserved := preservedDNSRecordSpec{Origin: src.Spec.Origin, Entries: src.Spec.Entries}
	preservedRaw, err := json.Marshal(preserved)
	if err != nil {
		return fmt.Errorf("marshal v1alpha2-only DNSRecordSpec for %s/%s: %w", src.Namespace, src.Name, err)
	}
	if dst.Annotations == nil {
		dst.Annotations = make(map[string]string, 1)
	}
	dst.Annotations[annotationV1Alpha2DNSRecordSpec] = string(preservedRaw)
	// v1alpha1 has no SourceType for manual records; force empty so the
	// conversion does not produce an invalid v1alpha1 object.
	if src.Spec.Origin == v1alpha2.DNSRecordOriginManual {
		dst.Spec.SourceType = ""
	}

	for _, e := range src.Status.Endpoints {
		dst.Status.Endpoints = append(dst.Status.Endpoints, EndpointStatus{
			DNSName:    e.DNSName,
			RecordType: e.RecordType,
			Targets:    e.Targets,
			TTL:        e.TTL,
			Labels:     e.Labels,
			SyncStatus: string(e.SyncStatus),
			LastSeen:   e.LastSeen,
		})
	}
	dst.Status.EndpointsHash = src.Status.EndpointsHash
	dst.Status.LastReconcileTime = src.Status.LastReconcileTime
	dst.Status.Conditions = src.Status.Conditions
	return nil
}
