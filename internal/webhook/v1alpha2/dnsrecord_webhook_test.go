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

package v1alpha2_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	webhookv1alpha2 "github.com/golgoth31/sreportal/internal/webhook/v1alpha2"
)

const testControllerSA = "system:serviceaccount:sreportal-system:sreportal-controller-manager"

func ctxWithUser(username string) context.Context {
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UserInfo: authenticationv1.UserInfo{Username: username},
		},
	}
	return admission.NewContextWithRequest(context.Background(), req)
}

// newFakeClient builds a controller-runtime fake client with the v1alpha1 and
// v1alpha2 schemes registered. v1alpha1 is needed because Portal has no
// v1alpha2 type — the DNSRecord webhook's portalRef-existence check looks up
// sreportalv1alpha1.Portal.
func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	g := NewWithT(t)
	g.Expect(sreportalv1alpha1.AddToScheme(s)).To(Succeed())
	g.Expect(sreportalv1alpha2.AddToScheme(s)).To(Succeed())
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
}

// newDNS constructs a minimal DNS object for seeding the fake client.
func newDNS() *sreportalv1alpha2.DNS {
	return &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tDNSName,
			Namespace: tNamespace,
		},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: tPortalMain,
		},
	}
}

// newPortal constructs a minimal Portal object for seeding the fake client.
func newPortal() *sreportalv1alpha1.Portal {
	return &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tPortalMain,
			Namespace: tNamespace,
		},
		Spec: sreportalv1alpha1.PortalSpec{
			Title: "Main Portal",
		},
	}
}

// validOwnerRef returns a controller ownerReference pointing to a DNS CR,
// matching the shape produced by controllerutil.SetControllerReference.
func validOwnerRef() metav1.OwnerReference {
	isController := true
	blockOwnerDeletion := true
	return metav1.OwnerReference{
		APIVersion:         sreportalv1alpha2.GroupVersion.String(),
		Kind:               "DNS",
		Name:               tDNSName,
		UID:                tDNSUID,
		Controller:         &isController,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
}

const (
	tNamespace = "default"
	tDNSName   = "main-dns"
	tDNSUID    = types.UID("dns-uid-1234")
)

// --- Existing tests: negative cases (short-circuit before ownerRef) ---
// These tests don't need ownerRef seeding because their errors are triggered
// by Origin/SourceType/Entries checks that run before the ownerRef block.

func TestDNSRecordWebhook_AutoRequiresSourceType(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: tRecordIngress},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef: tPortalMain,
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("sourceType is required"))
}

func TestDNSRecordWebhook_ManualRequiresEntries(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: tRecordManual},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("entries"))
}

// TestDNSRecordWebhook_UnknownOriginRejected guards defense-in-depth: even if
// the CRD enum/CEL markers regress, the webhook must reject an unknown origin
// rather than fall open and skip the controllerSA gate.
func TestDNSRecordWebhook_UnknownOriginRejected(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), testControllerSA)
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: tRecordManual},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    "bogus",
			PortalRef: tPortalMain,
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.origin must be"))
}

func TestDNSRecordWebhook_ManualRejectsSourceType(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: tRecordManual},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
			Entries:    []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("sourceType must be empty"))
}

func TestDNSRecordWebhook_OriginImmutable(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), testControllerSA)
	old := &sreportalv1alpha2.DNSRecord{
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	newR := &sreportalv1alpha2.DNSRecord{
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateUpdate(ctxWithUser(testControllerSA), old, newR)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("origin is immutable"))
}

func TestDNSRecordWebhook_PortalRefImmutable(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "")
	old := &sreportalv1alpha2.DNSRecord{
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	newR := &sreportalv1alpha2.DNSRecord{
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalOther,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateUpdate(context.Background(), old, newR)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("portalRef is immutable"))
}

func TestDNSRecordWebhook_AutoBlockedForNonControllerSA(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), testControllerSA)
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: tRecordIngress},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateCreate(ctxWithUser("kubectl-user"), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("reserved for the operator controller"))
}

// --- Existing tests: positive cases (need ownerRef + seeded DNS) ---

func TestDNSRecordWebhook_AutoValidCreate(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), testControllerSA)
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef()},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateCreate(ctxWithUser(testControllerSA), r)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestDNSRecordWebhook_ManualValidCreate(t *testing.T) {
	g := NewWithT(t)
	// A legitimate manual record is standalone: no DNS controller ownerReference
	// (rejected on create since 6-A), just a portalRef pointing at an existing
	// Portal.
	portal := newPortal()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, portal), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tRecordManual,
			Namespace: tNamespace,
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestDNSRecordWebhook_ValidUpdate(t *testing.T) {
	g := NewWithT(t)
	// A standalone manual record (no DNS owner) is updated to add an entry.
	portal := newPortal()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, portal), "")
	old := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Namespace: tNamespace},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	newR := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Namespace: tNamespace},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}, {FQDN: "b.example.com"}},
		},
	}
	_, err := v.ValidateUpdate(context.Background(), old, newR)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestDNSRecordWebhook_DeleteNoOp(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "")
	r := &sreportalv1alpha2.DNSRecord{
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateDelete(context.Background(), r)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestDNSRecordWebhook_AutoAllowedForControllerSA(t *testing.T) {
	g := NewWithT(t)
	controllerSA := testControllerSA
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), controllerSA)
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef()},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateCreate(ctxWithUser(controllerSA), r)
	g.Expect(err).NotTo(HaveOccurred())
}

// --- New tests: ownerRef validation (§8.2) ---

func TestDNSRecordWebhook_AutoRejectsMissingOwnerRef(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), testControllerSA)
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: tRecordIngress, Namespace: tNamespace},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateCreate(ctxWithUser(testControllerSA), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("required when spec.origin=auto"))
}

func TestDNSRecordWebhook_RejectsMultipleDNSControllerOwnerRefs(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), testControllerSA)
	ref2 := validOwnerRef()
	ref2.Name = "second-dns"
	ref2.UID = "dns-uid-second"
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef(), ref2},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateCreate(ctxWithUser(testControllerSA), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("at most one ownerReference"))
}

func TestDNSRecordWebhook_RejectsOwnerRefWithoutBlockOwnerDeletion(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), testControllerSA)
	ref := validOwnerRef()
	ref.BlockOwnerDeletion = nil
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{ref},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateCreate(ctxWithUser(testControllerSA), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("blockOwnerDeletion=true"))
}

func TestDNSRecordWebhook_RejectsDanglingOwnerDNS(t *testing.T) {
	g := NewWithT(t)
	// fake client has no DNS objects → owner lookup returns NotFound.
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), testControllerSA)
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef()},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateCreate(ctxWithUser(testControllerSA), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not found in namespace"))
}

func TestDNSRecordWebhook_RejectsPortalRefMismatch(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS() // owner DNS has spec.portalRef=tPortalMain.
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), testControllerSA)
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef()},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalOther,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateCreate(ctxWithUser(testControllerSA), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("must match owner DNS spec.portalRef"))
}

func TestDNSRecordWebhook_ManualWithoutOwnerRefAccepted(t *testing.T) {
	g := NewWithT(t)
	portal := newPortal()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, portal), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: tRecordManual, Namespace: tNamespace},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestDNSRecordWebhook_ManualWithoutOwnerRefRejectedWhenPortalMissing(t *testing.T) {
	g := NewWithT(t)
	// fake client has no Portal objects → portalRef lookup returns NotFound.
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: tRecordManual, Namespace: tNamespace},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("referenced portal"))
	g.Expect(err.Error()).To(ContainSubstring("not found in namespace"))
}

// TestDNSRecordWebhook_ManualWithOwnerRefRejectedOnCreate verifies the create
// gate: a manual record may not declare a DNS controller ownerReference at
// creation, regardless of whether the owner DNS exists. This blocks the abuse
// vector where an author binds a manual record to a DNS it doesn't own.
func TestDNSRecordWebhook_ManualWithOwnerRefRejectedOnCreate(t *testing.T) {
	g := NewWithT(t)
	// Owner DNS exists, yet a manual record still may not adopt it on create.
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordManual,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef()},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("must not declare a controller ownerReference"))
}

// TestDNSRecordWebhook_ManualAdoptionViaUpdateRejected verifies the blanket
// ban also closes the adoption-via-update vector: a standalone manual record
// cannot acquire a DNS controller ownerReference on update.
func TestDNSRecordWebhook_ManualAdoptionViaUpdateRejected(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), "")
	old := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Namespace: tNamespace},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	newR := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef()},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateUpdate(context.Background(), old, newR)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("must not declare a controller ownerReference"))
}

func TestDNSRecordWebhook_UpdateOwnerRefRemovalRejected(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), "")
	old := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef()},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	newR := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Namespace: tNamespace},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateUpdate(context.Background(), old, newR)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cannot be removed"))
}

func TestDNSRecordWebhook_UpdateReparentingRejected(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), "")
	other := validOwnerRef()
	other.Name = "team-b-dns"
	other.UID = "dns-uid-team-b"
	old := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef()},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	newR := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{other},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateUpdate(context.Background(), old, newR)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cannot re-parent"))
}

func TestDNSRecordWebhook_AutoRejectedWhenNoSAConfigured(t *testing.T) {
	g := NewWithT(t)
	// Fail closed: empty controllerSA must refuse every origin=auto write,
	// not silently allow them. Mirrors the cmd/main.go contract that the
	// operator refuses to start without SREPORTAL_CONTROLLER_SA when
	// webhooks are enabled.
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef()},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateCreate(ctxWithUser("any-user"), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("SREPORTAL_CONTROLLER_SA"))
}

func TestDNSRecordWebhook_AutoAllowsEntriesFromControllerSA(t *testing.T) {
	g := NewWithT(t)
	controllerSA := testControllerSA
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), controllerSA)
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef()},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
			Entries: []sreportalv1alpha2.DNSRecordEntry{
				{FQDN: tFQDNAPIExamp, RecordType: "A", Targets: []string{"1.2.3.4"}},
			},
		},
	}
	_, err := v.ValidateCreate(ctxWithUser(controllerSA), r)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestDNSRecordWebhook_AutoRejectsEntriesFromNonControllerSA(t *testing.T) {
	g := NewWithT(t)
	controllerSA := testControllerSA
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), controllerSA)
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef()},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
			Entries: []sreportalv1alpha2.DNSRecordEntry{
				{FQDN: tFQDNAPIExamp, RecordType: "A", Targets: []string{"1.2.3.4"}},
			},
		},
	}
	_, err := v.ValidateCreate(ctxWithUser("kubectl-user"), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("reserved for the operator controller"))
}

// autoRecordWithEntry returns an origin=auto DNSRecord with a single entry,
// shared by the ValidateUpdate SA-check tests below.
func autoRecordWithEntry(target string) *sreportalv1alpha2.DNSRecord {
	return &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef()},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
			Entries: []sreportalv1alpha2.DNSRecordEntry{
				{FQDN: tFQDNAPIExamp, RecordType: "A", Targets: []string{target}},
			},
		},
	}
}

// TestDNSRecordWebhook_AutoUpdateBlocksNonControllerSA closes the Update-path
// gap: ValidateUpdate must enforce the controllerSA gate, otherwise a human
// could not Create an origin=auto record but could mutate one after the
// operator created it.
func TestDNSRecordWebhook_AutoUpdateBlocksNonControllerSA(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), testControllerSA)
	old := autoRecordWithEntry("1.2.3.4")
	newR := autoRecordWithEntry("9.9.9.9")
	_, err := v.ValidateUpdate(ctxWithUser("kubectl-user"), old, newR)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("reserved for the operator controller"))
}

// TestDNSRecordWebhook_AutoUpdateAllowedForControllerSA is the positive
// counterpart: the operator must still be able to update its own auto records.
func TestDNSRecordWebhook_AutoUpdateAllowedForControllerSA(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS()
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), testControllerSA)
	old := autoRecordWithEntry("1.2.3.4")
	newR := autoRecordWithEntry("9.9.9.9")
	_, err := v.ValidateUpdate(ctxWithUser(testControllerSA), old, newR)
	g.Expect(err).NotTo(HaveOccurred())
}
