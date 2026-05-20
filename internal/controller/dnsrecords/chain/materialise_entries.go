// internal/controller/dnsrecords/chain/materialise_entries.go
package chain

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// MaterialiseEntriesHandler converts DNSRecord.Spec.Entries into
// status.Endpoints and refreshes status.EndpointsHash. It is the single
// projection step shared by auto and manual records — auto records receive
// their entries from the DNS controller writing spec.entries; manual records
// have entries set by the user.
//
// The handler persists status changes itself via Status().Patch when the
// endpoints hash or observedGeneration moves. Downstream handlers
// (ResolveDNS, ProjectStore) can short-circuit without losing the
// materialisation step.
type MaterialiseEntriesHandler struct {
	client client.Client
}

// NewMaterialiseEntriesHandler returns a new MaterialiseEntriesHandler.
// The client is used to patch DNSRecord status; pass nil only in tests that
// do not exercise persistence (the handler then mutates rc.Resource only).
func NewMaterialiseEntriesHandler(c client.Client) *MaterialiseEntriesHandler {
	return &MaterialiseEntriesHandler{client: c}
}

// Handle materialises spec.entries to status.Endpoints with a fresh
// LastSeen, recomputes EndpointsHash, and stamps LastReconcileTime. It is
// origin-agnostic. When spec.entries is empty, the status is cleared.
func (h *MaterialiseEntriesHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNSRecord, ChainData]) error {
	record := rc.Resource
	base := record.DeepCopy()

	now := metav1.Now()
	endpoints := make([]v1alpha2.EndpointStatus, 0, len(record.Spec.Entries))

	for _, e := range record.Spec.Entries {
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

	record.Status.Endpoints = endpoints
	record.Status.LastReconcileTime = &now
	if len(endpoints) == 0 {
		record.Status.EndpointsHash = ""
	} else {
		record.Status.EndpointsHash = adapter.EndpointStatusHashV2(endpoints)
	}
	record.Status.ObservedGeneration = record.Generation

	if h.client == nil {
		return nil
	}
	if base.Status.EndpointsHash == record.Status.EndpointsHash &&
		base.Status.ObservedGeneration == record.Status.ObservedGeneration {
		return nil
	}
	if err := h.client.Status().Patch(ctx, record, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch DNSRecord status: %w", err)
	}
	return nil
}
