// internal/controller/dnsrecords/chain/resolve_dns_test.go
package chain_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// stubResolver fails every lookup so endpoints get the "notfound" SyncStatus.
type stubResolver struct{ hosts map[string][]string }

func (r *stubResolver) LookupHost(_ context.Context, fqdn string) ([]string, error) {
	addrs, ok := r.hosts[fqdn]
	if !ok {
		return nil, fmt.Errorf("no such host: %s", fqdn)
	}
	return addrs, nil
}

func (r *stubResolver) LookupCNAME(_ context.Context, fqdn string) (string, error) {
	return "", fmt.Errorf("no CNAME for: %s", fqdn)
}

func newSchemeWithDNSRecord(g *WithT) *runtime.Scheme {
	scheme := runtime.NewScheme()
	g.Expect(v1alpha2.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func newRecordWithEndpoint(targets ...string) *v1alpha2.DNSRecord {
	return &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "main-svc", Namespace: tNsDefault},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:     v1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: "service",
		},
		Status: v1alpha2.DNSRecordStatus{
			Endpoints: []v1alpha2.EndpointStatus{
				{DNSName: "svc.example.com", RecordType: "A", Targets: targets, LastSeen: metav1.Now()},
			},
		},
	}
}

func TestResolveDNSHandler_SkipsWhenDisableDNSCheckTrue(t *testing.T) {
	g := NewWithT(t)
	record := newRecordWithEndpoint("1.2.3.4")
	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: record,
		Data:     chain.ChainData{ResourceKey: tKeyMainSvc, DisableDNSCheck: true},
	}

	h := chain.NewResolveDNSHandler(nil, nil)
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints[0].SyncStatus).To(BeEmpty())
}

func TestResolveDNSHandler_SkipsWhenResolverNil(t *testing.T) {
	g := NewWithT(t)
	record := newRecordWithEndpoint("1.2.3.4")
	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: record,
		Data:     chain.ChainData{ResourceKey: tKeyMainSvc},
	}

	h := chain.NewResolveDNSHandler(nil, nil)
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints[0].SyncStatus).To(BeEmpty())
}

func TestResolveDNSHandler_SkipsWhenNoEndpoints(t *testing.T) {
	g := NewWithT(t)
	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "main-svc", Namespace: tNsDefault},
		Spec:       v1alpha2.DNSRecordSpec{Origin: v1alpha2.DNSRecordOriginAuto, PortalRef: tPortalMain, SourceType: "service"},
	}
	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: record,
		Data:     chain.ChainData{ResourceKey: tKeyMainSvc},
	}

	scheme := newSchemeWithDNSRecord(g)
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1alpha2.DNSRecord{}).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourcePatch: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
				return fmt.Errorf("patch should not be called when there are no endpoints")
			},
		}).
		Build()

	g.Expect(chain.NewResolveDNSHandler(cli, &stubResolver{}).Handle(context.Background(), rc)).To(Succeed())
}

func TestResolveDNSHandler_SetsSyncStatusOnResolverError(t *testing.T) {
	g := NewWithT(t)
	record := newRecordWithEndpoint("1.2.3.4")
	scheme := newSchemeWithDNSRecord(g)
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1alpha2.DNSRecord{}).
		WithObjects(record).
		Build()

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: record,
		Data:     chain.ChainData{ResourceKey: tKeyMainSvc},
	}

	// Empty hosts map → LookupHost returns error → CheckFQDN reports "notfound".
	g.Expect(chain.NewResolveDNSHandler(cli, &stubResolver{}).Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints[0].SyncStatus).NotTo(BeEmpty())
	g.Expect(string(record.Status.Endpoints[0].SyncStatus)).To(Equal("notavailable"))
}

func TestResolveDNSHandler_WrapsPatchFailure(t *testing.T) {
	g := NewWithT(t)
	record := newRecordWithEndpoint("1.2.3.4")
	scheme := newSchemeWithDNSRecord(g)
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1alpha2.DNSRecord{}).
		WithObjects(record).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourcePatch: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
				return fmt.Errorf("simulated patch failure")
			},
		}).
		Build()

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: record,
		Data:     chain.ChainData{ResourceKey: tKeyMainSvc},
	}

	err := chain.NewResolveDNSHandler(cli, &stubResolver{}).Handle(context.Background(), rc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("patch DNSRecord status"))
	g.Expect(err.Error()).To(ContainSubstring("simulated patch failure"))
}
