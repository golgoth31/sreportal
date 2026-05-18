package main

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMigrate_PartialFailure_KeepsAnnotation_NonZeroExit(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme()
	dns := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p1",
			Namespace: "ns",
			Annotations: map[string]string{
				annotationV1Alpha1Groups: `[` +
					`{"name":"Apps","entries":[{"fqdn":"a.example.com"}]},` +
					`{"name":"Other","entries":[{"fqdn":"b.example.com"}]}` +
					`]`,
			},
		},
		Spec: v1alpha2.DNSSpec{PortalRef: "p1"},
	}
	// Pre-create a colliding record so the second loop iteration errors with AlreadyExists.
	// Use a distinct slug ("apps") to collide with the *first* group, then force a different
	// failure on the second group via the interceptor.
	existing := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "p1-manual-apps", Namespace: "ns"},
		Spec:       v1alpha2.DNSRecordSpec{Origin: v1alpha2.DNSRecordOriginManual, PortalRef: "p1"},
	}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dns, existing).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if rec, ok := obj.(*v1alpha2.DNSRecord); ok && rec.Name == "p1-manual-other" {
					return errFakeBoom
				}
				return c.Create(ctx, obj, opts...)
			},
		}).
		Build()

	sum, err := Migrate(context.Background(), cli, false)
	g.Expect(err).To(HaveOccurred())
	g.Expect(sum.Failures).To(BeNumerically(">", 0))

	var after v1alpha2.DNS
	g.Expect(cli.Get(context.Background(), client.ObjectKey{Name: "p1", Namespace: "ns"}, &after)).To(Succeed())
	g.Expect(after.Annotations).To(HaveKey(annotationV1Alpha1Groups))
}

func TestMigrate_DryRun_PassesDryRunAll(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme()
	dns := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p1",
			Namespace: "ns",
			Annotations: map[string]string{
				annotationV1Alpha1Groups: `[{"name":"Apps","entries":[{"fqdn":"a.example.com"}]}]`,
			},
		},
		Spec: v1alpha2.DNSSpec{PortalRef: "p1"},
	}
	var sawDryRun bool
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dns).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				for _, o := range opts {
					if o == client.DryRunAll {
						sawDryRun = true
					}
				}
				return c.Create(ctx, obj, opts...)
			},
		}).
		Build()

	sum, err := Migrate(context.Background(), cli, true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sum.Created).To(Equal(1))
	g.Expect(sawDryRun).To(BeTrue())

	// Annotation must still be present in dry-run mode.
	var after v1alpha2.DNS
	g.Expect(cli.Get(context.Background(), client.ObjectKey{Name: "p1", Namespace: "ns"}, &after)).To(Succeed())
	g.Expect(after.Annotations).To(HaveKey(annotationV1Alpha1Groups))
}

var errFakeBoom = &fakeBoom{}

type fakeBoom struct{}

func (e *fakeBoom) Error() string { return "boom" }
