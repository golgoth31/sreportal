// Package v1alpha2 contains API Schema definitions for the sreportal v1alpha2 API group.
// +kubebuilder:object:generate=true
// +groupName=sreportal.io
package v1alpha2

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion  = schema.GroupVersion{Group: "sreportal.io", Version: "v1alpha2"}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion} //nolint:staticcheck // SA1019: kubebuilder scaffold
	AddToScheme   = SchemeBuilder.AddToScheme
)
