// internal/controller/dnsrecords/chain/materialise_manual.go
package chain

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// MaterialiseManualEntriesHandler converts DNSRecordSpec.Entries into EndpointStatus
// entries on the resource status for manual-origin DNSRecord resources.
type MaterialiseManualEntriesHandler struct{}

// NewMaterialiseManualEntriesHandler returns a new MaterialiseManualEntriesHandler.
func NewMaterialiseManualEntriesHandler() *MaterialiseManualEntriesHandler {
	return &MaterialiseManualEntriesHandler{}
}

// Handle converts spec.entries to status.endpoints for manual-origin records.
// It is a no-op for auto-origin records.
func (h *MaterialiseManualEntriesHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNSRecord, ChainData]) error {
	if rc.Resource.Spec.Origin != v1alpha2.DNSRecordOriginManual {
		return nil
	}

	now := metav1.Now()
	endpoints := make([]v1alpha2.EndpointStatus, 0, len(rc.Resource.Spec.Entries))

	for _, e := range rc.Resource.Spec.Entries {
		rt := e.RecordType
		if rt == "" {
			rt = "A"
		}

		var labels map[string]string
		if e.Group != "" {
			labels = map[string]string{"sreportal.io/group": e.Group}
		}

		endpoints = append(endpoints, v1alpha2.EndpointStatus{
			DNSName:    e.FQDN,
			RecordType: rt,
			Targets:    e.Targets,
			Labels:     labels,
			LastSeen:   now,
		})
	}

	rc.Resource.Status.Endpoints = endpoints
	rc.Resource.Status.LastReconcileTime = &now

	return nil
}
