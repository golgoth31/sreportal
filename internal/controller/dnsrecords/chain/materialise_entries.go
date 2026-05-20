// internal/controller/dnsrecords/chain/materialise_entries.go
package chain

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// MaterialiseEntriesHandler converts DNSRecord.Spec.Entries into
// status.Endpoints and refreshes status.EndpointsHash. It is the single
// projection step shared by auto and manual records — auto records receive
// their entries from the DNS controller writing spec.entries; manual records
// have entries set by the user.
type MaterialiseEntriesHandler struct{}

// NewMaterialiseEntriesHandler returns a new MaterialiseEntriesHandler.
func NewMaterialiseEntriesHandler() *MaterialiseEntriesHandler {
	return &MaterialiseEntriesHandler{}
}

// Handle materialises spec.entries to status.Endpoints with a fresh
// LastSeen, recomputes EndpointsHash, and stamps LastReconcileTime. It is
// origin-agnostic. When spec.entries is empty, the status is cleared.
func (h *MaterialiseEntriesHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNSRecord, ChainData]) error {
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
	if len(endpoints) == 0 {
		rc.Resource.Status.EndpointsHash = ""
	} else {
		rc.Resource.Status.EndpointsHash = adapter.EndpointStatusHashV2(endpoints)
	}
	return nil
}
