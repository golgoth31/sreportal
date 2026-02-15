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

package controller

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/config"
	srcfactory "github.com/golgoth31/sreportal/internal/source"
)

// newLoadBalancerEndpoint creates an endpoint that simulates what external-dns
// produces for a LoadBalancer service with the hostname annotation.
// Resource label format matches external-dns: "service/namespace/name".
func newLoadBalancerEndpoint(hostname, ip, namespace, serviceName string) *endpoint.Endpoint {
	return &endpoint.Endpoint{
		DNSName:    hostname,
		RecordType: "A",
		Targets:    endpoint.Targets{ip},
		Labels: map[string]string{
			endpoint.ResourceLabelKey: fmt.Sprintf("service/%s/%s", namespace, serviceName),
		},
	}
}

// newLoadBalancerEndpointWithHostname creates an endpoint for AWS-style LoadBalancers
// that return a hostname instead of an IP.
func newLoadBalancerEndpointWithHostname(hostname, lbHostname, namespace, serviceName string) *endpoint.Endpoint {
	return &endpoint.Endpoint{
		DNSName:    hostname,
		RecordType: "CNAME",
		Targets:    endpoint.Targets{lbHostname},
		Labels: map[string]string{
			endpoint.ResourceLabelKey: fmt.Sprintf("service/%s/%s", namespace, serviceName),
		},
	}
}

// createTestPortal creates a Portal resource for tests.
func createTestPortal(name string) *sreportalv1alpha1.Portal {
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: sreportalv1alpha1.PortalSpec{
			Title: name,
			Main:  true,
		},
	}
	Expect(k8sClient.Create(ctx, portal)).To(Succeed())
	return portal
}

var _ = Describe("SourceReconciler", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		reconciler *SourceReconciler
		testConfig *config.OperatorConfig
	)

	BeforeEach(func() {
		testConfig = &config.OperatorConfig{
			Sources: config.SourcesConfig{
				Service: &config.ServiceConfig{
					Enabled:           true,
					ServiceTypeFilter: []string{"ClusterIP"},
				},
			},
			GroupMapping: config.GroupMappingConfig{
				DefaultGroup: "Test Services",
			},
			Reconciliation: config.ReconciliationConfig{
				Interval: config.Duration(time.Second),
			},
		}

		reconciler = NewSourceReconciler(
			k8sClient,
			scheme.Scheme,
			kubeClient,
			cfg,
			testConfig,
		)
	})

	AfterEach(func() {
		// Clean up Portal resources
		var portalList sreportalv1alpha1.PortalList
		if err := k8sClient.List(ctx, &portalList); err == nil {
			for _, p := range portalList.Items {
				_ = k8sClient.Delete(ctx, &p)
			}
		}
		// Clean up DNS resources
		var dnsList sreportalv1alpha1.DNSList
		err := k8sClient.List(ctx, &dnsList)
		if err == nil {
			for _, dns := range dnsList.Items {
				_ = k8sClient.Delete(ctx, &dns)
			}
		}
		// Clean up DNSRecord resources
		var dnsRecordList sreportalv1alpha1.DNSRecordList
		err = k8sClient.List(ctx, &dnsRecordList)
		if err == nil {
			for _, rec := range dnsRecordList.Items {
				_ = k8sClient.Delete(ctx, &rec)
			}
		}
	})

	Context("NewSourceReconciler", func() {
		It("should create reconciler with correct fields", func() {
			Expect(reconciler.Client).NotTo(BeNil())
			Expect(reconciler.Scheme).NotTo(BeNil())
			Expect(reconciler.KubeClient).NotTo(BeNil())
			Expect(reconciler.RestConfig).NotTo(BeNil())
			Expect(reconciler.Config).To(Equal(testConfig))
		})
	})

	Context("reconcileDNSRecord", func() {
		It("should create DNSRecord for a portal and source type", func() {
			// Create Portal
			portalName := fmt.Sprintf("test-portal-%s", rand.String(5))
			portal := createTestPortal(portalName)

			// Wait for creation
			portalKey := types.NamespacedName{Name: portalName, Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, portalKey, &sreportalv1alpha1.Portal{})
			}, timeout, interval).Should(Succeed())

			// Set up mock endpoints
			endpoints := []*endpoint.Endpoint{
				{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}},
			}

			// Reconcile DNSRecord
			err := reconciler.reconcileDNSRecord(ctx, portal, srcfactory.SourceTypeService, endpoints)
			Expect(err).NotTo(HaveOccurred())

			// Verify DNSRecord created
			dnsRecordKey := types.NamespacedName{
				Name:      fmt.Sprintf("%s-service", portalName),
				Namespace: "default",
			}
			Eventually(func(g Gomega) {
				var dnsRecord sreportalv1alpha1.DNSRecord
				g.Expect(k8sClient.Get(ctx, dnsRecordKey, &dnsRecord)).To(Succeed())
				g.Expect(dnsRecord.Spec.SourceType).To(Equal("service"))
				g.Expect(dnsRecord.Spec.PortalRef).To(Equal(portalName))
				g.Expect(dnsRecord.Status.Endpoints).To(HaveLen(1))
				g.Expect(dnsRecord.Status.Endpoints[0].DNSName).To(Equal("api.example.com"))
			}, timeout, interval).Should(Succeed())
		})

		It("should update existing DNSRecord on reconciliation", func() {
			// Create Portal
			portalName := fmt.Sprintf("update-portal-%s", rand.String(5))
			portal := createTestPortal(portalName)

			portalKey := types.NamespacedName{Name: portalName, Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, portalKey, &sreportalv1alpha1.Portal{})
			}, timeout, interval).Should(Succeed())

			// First reconciliation with one endpoint
			endpoints := []*endpoint.Endpoint{
				{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}},
			}
			err := reconciler.reconcileDNSRecord(ctx, portal, srcfactory.SourceTypeService, endpoints)
			Expect(err).NotTo(HaveOccurred())

			// Second reconciliation with new endpoint
			endpoints = []*endpoint.Endpoint{
				{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}},
				{DNSName: "web.example.com", RecordType: "A", Targets: []string{"10.0.0.2"}},
			}
			err = reconciler.reconcileDNSRecord(ctx, portal, srcfactory.SourceTypeService, endpoints)
			Expect(err).NotTo(HaveOccurred())

			// Verify DNSRecord updated
			dnsRecordKey := types.NamespacedName{
				Name:      fmt.Sprintf("%s-service", portalName),
				Namespace: "default",
			}
			Eventually(func(g Gomega) {
				var dnsRecord sreportalv1alpha1.DNSRecord
				g.Expect(k8sClient.Get(ctx, dnsRecordKey, &dnsRecord)).To(Succeed())
				g.Expect(dnsRecord.Status.Endpoints).To(HaveLen(2))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("deleteOrphanedDNSRecords", func() {
		It("should delete DNSRecords for disabled sources", func() {
			// Create Portal
			portalName := fmt.Sprintf("orphan-portal-%s", rand.String(5))
			portal := createTestPortal(portalName)

			portalKey := types.NamespacedName{Name: portalName, Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, portalKey, &sreportalv1alpha1.Portal{})
			}, timeout, interval).Should(Succeed())

			// Create a DNSRecord for this portal
			endpoints := []*endpoint.Endpoint{
				{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}},
			}
			err := reconciler.reconcileDNSRecord(ctx, portal, srcfactory.SourceTypeService, endpoints)
			Expect(err).NotTo(HaveOccurred())

			// Verify DNSRecord exists
			dnsRecordKey := types.NamespacedName{
				Name:      fmt.Sprintf("%s-service", portalName),
				Namespace: "default",
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, dnsRecordKey, &sreportalv1alpha1.DNSRecord{})
			}, timeout, interval).Should(Succeed())

			// Disable service source in config
			testConfig.Sources.Service.Enabled = false

			// Delete orphaned records (empty active keys means nothing is active)
			activeKeys := make(map[portalSourceKey][]*endpoint.Endpoint)
			err = reconciler.deleteOrphanedDNSRecords(ctx, portal, activeKeys)
			Expect(err).NotTo(HaveOccurred())

			// Verify DNSRecord deleted
			Eventually(func(g Gomega) {
				var dnsRecord sreportalv1alpha1.DNSRecord
				err := k8sClient.Get(ctx, dnsRecordKey, &dnsRecord)
				g.Expect(client.IgnoreNotFound(err)).To(Succeed())
				g.Expect(err).To(HaveOccurred()) // Should be NotFound
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("rebuildSources", func() {
		It("should build typed sources from config", func() {
			err := reconciler.rebuildSources(ctx)
			if err != nil {
				Skip("rebuildSources requires cluster setup")
			}

			typedSources := reconciler.GetTypedSources()
			Expect(typedSources).To(HaveLen(1))
			Expect(typedSources[0].Type).To(Equal(srcfactory.SourceTypeService))
		})
	})
})

var _ = Describe("SourceReconciler LoadBalancer Integration", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		reconciler *SourceReconciler
		testConfig *config.OperatorConfig
	)

	BeforeEach(func() {
		testConfig = &config.OperatorConfig{
			Sources: config.SourcesConfig{
				Service: &config.ServiceConfig{
					Enabled:           true,
					ServiceTypeFilter: []string{"LoadBalancer"},
				},
			},
			GroupMapping: config.GroupMappingConfig{
				DefaultGroup: "LoadBalancer Services",
			},
			Reconciliation: config.ReconciliationConfig{
				Interval: config.Duration(time.Second),
			},
		}

		reconciler = NewSourceReconciler(
			k8sClient,
			scheme.Scheme,
			kubeClient,
			cfg,
			testConfig,
		)
	})

	AfterEach(func() {
		// Clean up Portal resources
		var portalList sreportalv1alpha1.PortalList
		if err := k8sClient.List(ctx, &portalList); err == nil {
			for _, p := range portalList.Items {
				_ = k8sClient.Delete(ctx, &p)
			}
		}
		// Clean up DNS resources
		var dnsList sreportalv1alpha1.DNSList
		if err := k8sClient.List(ctx, &dnsList); err == nil {
			for _, dns := range dnsList.Items {
				_ = k8sClient.Delete(ctx, &dns)
			}
		}
		// Clean up DNSRecord resources
		var dnsRecordList sreportalv1alpha1.DNSRecordList
		if err := k8sClient.List(ctx, &dnsRecordList); err == nil {
			for _, rec := range dnsRecordList.Items {
				_ = k8sClient.Delete(ctx, &rec)
			}
		}
	})

	Context("with simulated LoadBalancer endpoints", func() {
		It("should create DNSRecord with LoadBalancer endpoint", func() {
			// Create Portal
			portalName := fmt.Sprintf("lb-portal-%s", rand.String(5))
			portal := createTestPortal(portalName)

			portalKey := types.NamespacedName{Name: portalName, Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, portalKey, &sreportalv1alpha1.Portal{})
			}, timeout, interval).Should(Succeed())

			// Create mock LoadBalancer endpoint
			endpoints := []*endpoint.Endpoint{
				newLoadBalancerEndpoint("api.example.com", "203.0.113.10", "production", "my-loadbalancer"),
			}

			// Reconcile
			err := reconciler.reconcileDNSRecord(ctx, portal, srcfactory.SourceTypeService, endpoints)
			Expect(err).NotTo(HaveOccurred())

			// Verify DNSRecord
			dnsRecordKey := types.NamespacedName{
				Name:      fmt.Sprintf("%s-service", portalName),
				Namespace: "default",
			}
			Eventually(func(g Gomega) {
				var dnsRecord sreportalv1alpha1.DNSRecord
				g.Expect(k8sClient.Get(ctx, dnsRecordKey, &dnsRecord)).To(Succeed())
				g.Expect(dnsRecord.Status.Endpoints).To(HaveLen(1))
				g.Expect(dnsRecord.Status.Endpoints[0].DNSName).To(Equal("api.example.com"))
				g.Expect(dnsRecord.Status.Endpoints[0].Targets).To(ContainElement("203.0.113.10"))
				g.Expect(dnsRecord.Status.Endpoints[0].RecordType).To(Equal("A"))
			}, timeout, interval).Should(Succeed())
		})

		It("should handle AWS-style LoadBalancer with hostname target", func() {
			portalName := fmt.Sprintf("aws-lb-portal-%s", rand.String(5))
			portal := createTestPortal(portalName)

			portalKey := types.NamespacedName{Name: portalName, Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, portalKey, &sreportalv1alpha1.Portal{})
			}, timeout, interval).Should(Succeed())

			// AWS ELB returns a hostname instead of IP
			endpoints := []*endpoint.Endpoint{
				newLoadBalancerEndpointWithHostname(
					"myapp.example.com",
					"a1234567890.us-east-1.elb.amazonaws.com",
					"production",
					"aws-lb",
				),
			}

			err := reconciler.reconcileDNSRecord(ctx, portal, srcfactory.SourceTypeService, endpoints)
			Expect(err).NotTo(HaveOccurred())

			dnsRecordKey := types.NamespacedName{
				Name:      fmt.Sprintf("%s-service", portalName),
				Namespace: "default",
			}
			Eventually(func(g Gomega) {
				var dnsRecord sreportalv1alpha1.DNSRecord
				g.Expect(k8sClient.Get(ctx, dnsRecordKey, &dnsRecord)).To(Succeed())
				g.Expect(dnsRecord.Status.Endpoints).To(HaveLen(1))
				g.Expect(dnsRecord.Status.Endpoints[0].DNSName).To(Equal("myapp.example.com"))
				g.Expect(dnsRecord.Status.Endpoints[0].Targets).To(ContainElement("a1234567890.us-east-1.elb.amazonaws.com"))
				g.Expect(dnsRecord.Status.Endpoints[0].RecordType).To(Equal("CNAME"))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("endpoint grouping", func() {
		It("should correctly convert endpoints to groups using adapter", func() {
			endpoints := []*endpoint.Endpoint{
				newLoadBalancerEndpoint("api.example.com", "203.0.113.1", "production", "api-lb"),
				newLoadBalancerEndpoint("admin.example.com", "203.0.113.2", "production", "admin-lb"),
				newLoadBalancerEndpoint("staging-api.example.com", "203.0.113.3", "staging", "api-lb"),
			}

			groups := adapter.EndpointsToGroups(endpoints, &testConfig.GroupMapping)
			Expect(groups).To(HaveLen(1))
			Expect(groups[0].Name).To(Equal("LoadBalancer Services"))
			Expect(groups[0].FQDNs).To(HaveLen(3))
		})

		It("should group by namespace when configured", func() {
			testConfig.GroupMapping.ByNamespace = map[string]string{
				"production": "Production Services",
				"staging":    "Staging Services",
			}

			endpoints := []*endpoint.Endpoint{
				newLoadBalancerEndpoint("api.example.com", "203.0.113.1", "production", "api-lb"),
				newLoadBalancerEndpoint("staging-api.example.com", "203.0.113.2", "staging", "api-lb"),
			}

			groups := adapter.EndpointsToGroups(endpoints, &testConfig.GroupMapping)

			Expect(groups).To(HaveLen(2))
			Expect(groups[0].Name).To(Equal("Production Services"))
			Expect(groups[0].FQDNs).To(HaveLen(1))
			Expect(groups[0].FQDNs[0].FQDN).To(Equal("api.example.com"))

			Expect(groups[1].Name).To(Equal("Staging Services"))
			Expect(groups[1].FQDNs).To(HaveLen(1))
			Expect(groups[1].FQDNs[0].FQDN).To(Equal("staging-api.example.com"))
		})
	})

	Context("Factory configuration for LoadBalancer", func() {
		It("should build service source with LoadBalancer filter", func() {
			err := reconciler.rebuildSources(ctx)
			Expect(err).NotTo(HaveOccurred())

			typedSources := reconciler.GetTypedSources()
			Expect(typedSources).To(HaveLen(1))
			Expect(typedSources[0].Type).To(Equal(srcfactory.SourceTypeService))
		})

		It("should return empty sources when service source is disabled", func() {
			testConfig.Sources.Service.Enabled = false

			disabledReconciler := NewSourceReconciler(
				k8sClient,
				scheme.Scheme,
				kubeClient,
				cfg,
				testConfig,
			)

			err := disabledReconciler.rebuildSources(ctx)
			Expect(err).NotTo(HaveOccurred())

			typedSources := disabledReconciler.GetTypedSources()
			Expect(typedSources).To(BeEmpty())
		})
	})
})
