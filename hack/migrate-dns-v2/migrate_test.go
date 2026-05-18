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

// newAnnotatedDNS builds a v1alpha2.DNS in namespace "ns" with the given name,
// PortalRef and v1alpha1-groups annotation payload. Used by the strip-gate tests
// that only differ in name and JSON payload.
func newAnnotatedDNS(name, groupsJSON string) *v1alpha2.DNS {
	return &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "ns",
			Annotations: map[string]string{
				annotationV1Alpha1Groups: groupsJSON,
			},
		},
		Spec: v1alpha2.DNSSpec{PortalRef: name},
	}
}

// TestMigrate_SuccessStripsAnnotation verifies that when every group is
// successfully created and dryRun is false, the migration removes the
// annotationV1Alpha1Groups annotation from the DNS CR.
func TestMigrate_SuccessStripsAnnotation(t *testing.T) {
	g := NewWithT(t)
	dns := newAnnotatedDNS("p1", `[{"name":"Apps","entries":[{"fqdn":"a.example.com"}]}]`)
	cli := fake.NewClientBuilder().
		WithScheme(newScheme()).
		WithObjects(dns).
		Build()

	sum, err := Migrate(context.Background(), cli, false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sum.Created).To(Equal(1))
	g.Expect(sum.Failures).To(Equal(0))

	// Annotation must be removed after successful migration.
	var after v1alpha2.DNS
	g.Expect(cli.Get(context.Background(), client.ObjectKey{Name: "p1", Namespace: "ns"}, &after)).To(Succeed())
	g.Expect(after.Annotations).NotTo(HaveKey(annotationV1Alpha1Groups))
}

// TestMigrate_ZeroGroupCount verifies that a DNS CR whose annotation decodes
// to a slice with no non-empty groups does not panic and is reported as
// Skipped (groupCount == 0 means the strip-gate is never reached).
func TestMigrate_ZeroGroupCount(t *testing.T) {
	g := NewWithT(t)
	// Valid JSON but every group has no entries — groupCount stays 0.
	dns := newAnnotatedDNS("p2", `[{"name":"Empty","entries":[]}]`)
	cli := fake.NewClientBuilder().
		WithScheme(newScheme()).
		WithObjects(dns).
		Build()

	sum, err := Migrate(context.Background(), cli, false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sum.Failures).To(Equal(0))
	g.Expect(sum.Created).To(Equal(0))
	// No DNSRecord was created, so no annotation strip; annotation remains.
	var after v1alpha2.DNS
	g.Expect(cli.Get(context.Background(), client.ObjectKey{Name: "p2", Namespace: "ns"}, &after)).To(Succeed())
	// The annotation is intentionally left; the groupCount==0 gate prevents stripping.
	g.Expect(after.Annotations).To(HaveKey(annotationV1Alpha1Groups))
}

// TestMigrate_AllAlreadyExists verifies that when every per-DNS Create returns
// AlreadyExists the tool: (a) exits cleanly with no error, (b) counts each
// collision in AlreadyExist, and (c) still strips the annotation because
// AlreadyExists is counted in perDNSCreated (idempotent re-run semantics).
func TestMigrate_AllAlreadyExists(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme()

	dns := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p3",
			Namespace: "ns",
			Annotations: map[string]string{
				annotationV1Alpha1Groups: `[` +
					`{"name":"Apps","entries":[{"fqdn":"a.example.com"}]},` +
					`{"name":"Infra","entries":[{"fqdn":"b.example.com"}]}` +
					`]`,
			},
		},
		Spec: v1alpha2.DNSSpec{PortalRef: "p3"},
	}
	// Pre-create both DNSRecords so every Create returns AlreadyExists.
	existing1 := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "p3-manual-apps", Namespace: "ns"},
		Spec:       v1alpha2.DNSRecordSpec{Origin: v1alpha2.DNSRecordOriginManual, PortalRef: "p3"},
	}
	existing2 := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "p3-manual-infra", Namespace: "ns"},
		Spec:       v1alpha2.DNSRecordSpec{Origin: v1alpha2.DNSRecordOriginManual, PortalRef: "p3"},
	}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dns, existing1, existing2).
		Build()

	sum, err := Migrate(context.Background(), cli, false)
	g.Expect(err).NotTo(HaveOccurred(), "all-AlreadyExists must not be treated as an error")
	g.Expect(sum.Failures).To(Equal(0))
	g.Expect(sum.AlreadyExist).To(Equal(2))
	g.Expect(sum.Created).To(Equal(0))

	// Because AlreadyExists counts toward perDNSCreated, the strip gate fires
	// and the annotation must be removed.
	var after v1alpha2.DNS
	g.Expect(cli.Get(context.Background(), client.ObjectKey{Name: "p3", Namespace: "ns"}, &after)).To(Succeed())
	g.Expect(after.Annotations).NotTo(HaveKey(annotationV1Alpha1Groups))
}

var errFakeBoom = &fakeBoom{}

type fakeBoom struct{}

func (e *fakeBoom) Error() string { return "boom" }
