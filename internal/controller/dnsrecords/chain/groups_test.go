package chain_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// TestMaterialiseEntriesHandler_ReinjectsGroups verifies entry.Groups is
// re-injected into the status endpoint's sreportal.io/groups label so the
// read-side group mapping projects the FQDN into all its groups.
func TestMaterialiseEntriesHandler_ReinjectsGroups(t *testing.T) {
	g := NewWithT(t)
	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "auto-groups", Namespace: tNsDefault},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginAuto,
			PortalRef: tPortalMain,
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "a.example.com", RecordType: "A", Targets: []string{tIP1234}, Groups: []string{"Team A", "Shared"}},
			},
		},
	}
	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: record}
	g.Expect(chain.NewMaterialiseEntriesHandler(nil).Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints).To(HaveLen(1))
	g.Expect(record.Status.Endpoints[0].Labels["sreportal.io/groups"]).To(Equal("Team A,Shared"))
}
