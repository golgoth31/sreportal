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

package dnsrecords

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/external-dns/source/annotations"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	"github.com/golgoth31/sreportal/internal/log"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	Expect(log.Init(log.Config{
		Format: log.FormatRaw,
		Level:  log.LevelDebugValue,
		Output: GinkgoWriter,
	})).To(Succeed())
	ctrl.SetLogger(log.Default().ToLogr())
	annotations.SetAnnotationPrefix("external-dns.alpha.kubernetes.io/")

	ctx, cancel = context.WithCancel(context.TODO())

	var err error
	err = sreportalv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// Create a manager to get a proper client with caching and indexing
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"}, // Disable metrics to avoid port conflicts
	})
	Expect(err).NotTo(HaveOccurred())

	// Add field indexer for DNSRecord.spec.portalRef
	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.DNSRecord{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			dnsRecord := o.(*sreportalv1alpha1.DNSRecord)
			if dnsRecord.Spec.PortalRef == "" {
				return nil
			}
			return []string{dnsRecord.Spec.PortalRef}
		},
	)
	Expect(err).NotTo(HaveOccurred())

	// Add field indexer for DNS.spec.portalRef
	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.DNS{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			dns := o.(*sreportalv1alpha1.DNS)
			if dns.Spec.PortalRef == "" {
				return nil
			}
			return []string{dns.Spec.PortalRef}
		},
	)
	Expect(err).NotTo(HaveOccurred())

	// Add field indexer for Release.spec.portalRef
	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.Release{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			rel := o.(*sreportalv1alpha1.Release)
			if rel.Spec.PortalRef == "" {
				return nil
			}
			return []string{rel.Spec.PortalRef}
		},
	)
	Expect(err).NotTo(HaveOccurred())

	// Add field indexer for NetworkFlowDiscovery.spec.portalRef
	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.NetworkFlowDiscovery{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			nfd := o.(*sreportalv1alpha1.NetworkFlowDiscovery)
			if nfd.Spec.PortalRef == "" {
				return nil
			}
			return []string{nfd.Spec.PortalRef}
		},
	)
	Expect(err).NotTo(HaveOccurred())

	// Add field indexer for Alertmanager.spec.portalRef
	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.Alertmanager{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			am := o.(*sreportalv1alpha1.Alertmanager)
			if am.Spec.PortalRef == "" {
				return nil
			}
			return []string{am.Spec.PortalRef}
		},
	)
	Expect(err).NotTo(HaveOccurred())

	// Add field indexer for Component.spec.portalRef
	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.Component{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			comp := o.(*sreportalv1alpha1.Component)
			if comp.Spec.PortalRef == "" {
				return nil
			}
			return []string{comp.Spec.PortalRef}
		},
	)
	Expect(err).NotTo(HaveOccurred())

	// Add field indexer for Incident.spec.portalRef
	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.Incident{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			inc := o.(*sreportalv1alpha1.Incident)
			if inc.Spec.PortalRef == "" {
				return nil
			}
			return []string{inc.Spec.PortalRef}
		},
	)
	Expect(err).NotTo(HaveOccurred())

	// Add field indexer for Maintenance.spec.portalRef
	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.Maintenance{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			maint := o.(*sreportalv1alpha1.Maintenance)
			if maint.Spec.PortalRef == "" {
				return nil
			}
			return []string{maint.Spec.PortalRef}
		},
	)
	Expect(err).NotTo(HaveOccurred())

	// Start the manager in background
	go func() {
		defer GinkgoRecover()
		err := mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	// Use the manager's client which has the indexer
	k8sClient = mgr.GetClient()
	Expect(k8sClient).NotTo(BeNil())

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	Eventually(func() error {
		return testEnv.Stop()
	}, time.Minute, time.Second).Should(Succeed())
})

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		log.Default().Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}
