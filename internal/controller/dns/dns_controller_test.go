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

package dns

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// emptySourceReader is a no-op SourceEndpointReader for controller tests that
// do not exercise source lookups. LookupSourcesHandler fails loud on a nil
// reader (wiring bug detection), so tests must supply a non-nil stub.
type emptySourceReader struct{}

func (emptySourceReader) Lookup(_ registry.SourceType, _, _ string) ([]domainsource.EnrichedEndpoint, error) {
	return nil, nil
}

// Ready reports true so the "not synced yet" preserve gate is a no-op for tests
// that exercise the authoritatively-empty path.
func (emptySourceReader) Ready(_ registry.SourceType) bool { return true }

var _ = Describe("DNS Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When the DNS resource does not exist", func() {
		It("should not return an error", func() {
			controllerReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), emptySourceReader{}, nil)

			_, err := controllerReconciler.Reconcile(context.Background(), reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent",
					Namespace: tNsDefault,
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When reconciling an empty DNS resource (no DNSRecords)", func() {
		const resourceName = "test-dns-empty"
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: tNsDefault,
		}

		BeforeEach(func() {
			By("creating an empty v1alpha2 DNS resource")
			dns := &v1alpha2.DNS{}
			err := k8sClient.Get(ctx, typeNamespacedName, dns)
			if err != nil && errors.IsNotFound(err) {
				resource := &v1alpha2.DNS{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: tNsDefault,
					},
					Spec: v1alpha2.DNSSpec{
						PortalRef:    tPortalMain,
						GroupMapping: v1alpha2.GroupMappingSpec{DefaultGroup: tGroupServices},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &v1alpha2.DNS{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup the DNS resource")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile with empty groups and Ready condition", func() {
			controllerReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), emptySourceReader{}, nil)

			By("Reconciling and checking the DNS status is empty but has conditions")
			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				g.Expect(err).NotTo(HaveOccurred())

				var dns v1alpha2.DNS
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &dns)).To(Succeed())
				// TODO(Phase 9): replaced by readstore — Status.Groups removed from v1alpha2 DNSStatus
				// g.Expect(dns.Status.Groups).To(BeEmpty())
				g.Expect(dns.Status.LastReconcileTime).NotTo(BeNil())
				g.Expect(dns.Status.Conditions).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When DNS has associated DNSRecords with endpoints", func() {
		const (
			dnsName    = "test-dns-with-records"
			recordName = "test-dns-with-records-ingress"
		)
		ctx := context.Background()

		dnsNN := types.NamespacedName{Name: dnsName, Namespace: tNsDefault}
		recordNN := types.NamespacedName{Name: recordName, Namespace: tNsDefault}

		BeforeEach(func() {
			By("creating a v1alpha2 DNS resource")
			dns := &v1alpha2.DNS{}
			if err := k8sClient.Get(ctx, dnsNN, dns); err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &v1alpha2.DNS{
					ObjectMeta: metav1.ObjectMeta{Name: dnsName, Namespace: tNsDefault},
					Spec: v1alpha2.DNSSpec{
						PortalRef:    dnsName,
						GroupMapping: v1alpha2.GroupMappingSpec{DefaultGroup: tGroupServices},
					},
				})).To(Succeed())
			}

			By("creating a DNSRecord with spec entries")
			rec := &v1alpha2.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); err != nil && errors.IsNotFound(err) {
				rec = &v1alpha2.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: tNsDefault},
					Spec: v1alpha2.DNSRecordSpec{
						Origin:     v1alpha2.DNSRecordOriginAuto,
						SourceType: "ingress",
						PortalRef:  dnsName,
						Entries: []v1alpha2.DNSRecordEntry{
							{FQDN: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}},
						},
					},
				}
				Expect(k8sClient.Create(ctx, rec)).To(Succeed())
			}
		})

		AfterEach(func() {
			rec := &v1alpha2.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); err == nil {
				Expect(k8sClient.Delete(ctx, rec)).To(Succeed())
			}
			dns := &v1alpha2.DNS{}
			if err := k8sClient.Get(ctx, dnsNN, dns); err == nil {
				Expect(k8sClient.Delete(ctx, dns)).To(Succeed())
			}
		})

		It("should aggregate DNSRecord endpoints into DNS status groups", func() {
			controllerReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), emptySourceReader{}, nil)

			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: dnsNN})
				g.Expect(err).NotTo(HaveOccurred())

				var dns v1alpha2.DNS
				g.Expect(k8sClient.Get(ctx, dnsNN, &dns)).To(Succeed())
				// TODO(Phase 9): replaced by readstore — Status.Groups removed from v1alpha2 DNSStatus
				// g.Expect(dns.Status.Groups).NotTo(BeEmpty())
				g.Expect(dns.Status.LastReconcileTime).NotTo(BeNil())
				g.Expect(dns.Status.Conditions).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When reconciling a remote DNS resource", func() {
		const resourceName = "remote-test-portal"
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: tNsDefault,
		}

		BeforeEach(func() {
			By("creating a remote DNS resource")
			dns := &v1alpha2.DNS{}
			err := k8sClient.Get(ctx, typeNamespacedName, dns)
			if err != nil && errors.IsNotFound(err) {
				resource := &v1alpha2.DNS{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: tNsDefault,
					},
					Spec: v1alpha2.DNSSpec{
						PortalRef:    "test-portal",
						IsRemote:     true,
						GroupMapping: v1alpha2.GroupMappingSpec{DefaultGroup: tGroupServices},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &v1alpha2.DNS{}
			if err := k8sClient.Get(ctx, typeNamespacedName, resource); err == nil {
				By("Cleanup the DNS resource")
				_ = k8sClient.Delete(ctx, resource)
			}
		})

		It("should skip reconciliation without error", func() {
			controllerReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), emptySourceReader{}, nil)

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero(), "remote DNS should not be requeued by DNS controller")
		})
	})
})
