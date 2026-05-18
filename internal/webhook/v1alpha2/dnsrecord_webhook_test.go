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

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	webhookv1alpha2 "github.com/golgoth31/sreportal/internal/webhook/v1alpha2"
)

func ctxWithUser(username string) context.Context {
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UserInfo: authenticationv1.UserInfo{Username: username},
		},
	}
	return admission.NewContextWithRequest(context.Background(), req)
}

// newFakeClient builds a controller-runtime fake client with the v1alpha2 scheme registered.
func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	g := NewWithT(t)
	g.Expect(sreportalv1alpha2.AddToScheme(s)).To(Succeed())
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
}

// newDNS constructs a minimal DNS object for seeding the fake client.
func newDNS(name, namespace, portalRef string) *sreportalv1alpha2.DNS {
	return &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: portalRef,
		},
	}
}

// validOwnerRef returns a controller ownerReference pointing to a DNS CR.
func validOwnerRef(name string, uid types.UID) metav1.OwnerReference {
	isController := true
	return metav1.OwnerReference{
		APIVersion: sreportalv1alpha2.GroupVersion.String(),
		Kind:       "DNS",
		Name:       name,
		UID:        uid,
		Controller: &isController,
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

func TestDNSRecordWebhook_AutoRejectsEntries(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: tRecordIngress},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
			Entries:    []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("entries must be empty"))
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
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateUpdate(context.Background(), old, newR)
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
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "system:serviceaccount:sreportal-system:sreportal-controller-manager")
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
	dns := newDNS(tDNSName, tNamespace, tPortalMain)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef(tDNSName, tDNSUID)},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestDNSRecordWebhook_ManualValidCreate(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS(tDNSName, tNamespace, tPortalMain)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordManual,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef(tDNSName, tDNSUID)},
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
	dns := newDNS(tDNSName, tNamespace, tPortalMain)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), "")
	ownerRef := validOwnerRef(tDNSName, tDNSUID)
	old := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
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
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
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
	controllerSA := "system:serviceaccount:sreportal-system:sreportal-controller-manager"
	dns := newDNS(tDNSName, tNamespace, tPortalMain)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), controllerSA)
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef(tDNSName, tDNSUID)},
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

func TestDNSRecordWebhook_AutoAllowedWhenNoSAConfigured(t *testing.T) {
	g := NewWithT(t)
	// When controllerSA is empty, any user can create auto records (SA check disabled).
	dns := newDNS(tDNSName, tNamespace, tPortalMain)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordIngress,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef(tDNSName, tDNSUID)},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: tSourceIngress,
		},
	}
	_, err := v.ValidateCreate(ctxWithUser("any-user"), r)
	g.Expect(err).NotTo(HaveOccurred())
}

// --- New tests: ownerRef validation (6 cases from spec) ---

// Case 1: rejects DNSRecord with no ownerReferences.
func TestDNSRecordWebhook_OwnerRef_RejectsNoOwnerRefs(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tRecordManual,
			Namespace: tNamespace,
			// No OwnerReferences
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("exactly one controller ownerReference"))
}

// Case 2: rejects DNSRecord with two controller ownerRefs.
func TestDNSRecordWebhook_OwnerRef_RejectsTwoControllerRefs(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "")
	isController := true
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tRecordManual,
			Namespace: tNamespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: sreportalv1alpha2.GroupVersion.String(),
					Kind:       "DNS",
					Name:       "dns-1",
					UID:        "uid-1",
					Controller: &isController,
				},
				{
					APIVersion: sreportalv1alpha2.GroupVersion.String(),
					Kind:       "DNS",
					Name:       "dns-2",
					UID:        "uid-2",
					Controller: &isController,
				},
			},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("exactly one controller ownerReference"))
}

// Case 2b: two controller ownerRefs of mixed kinds (one valid DNS, one wrong)
// must still surface as "exactly one controller ownerReference", not the wrong-Kind error.
func TestDNSRecordWebhook_OwnerRef_RejectsTwoControllerRefsMixedKinds(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "")
	isController := true
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tRecordManual,
			Namespace: tNamespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: sreportalv1alpha2.GroupVersion.String(),
					Kind:       "DNS",
					Name:       tDNSName,
					UID:        tDNSUID,
					Controller: &isController,
				},
				{
					APIVersion: sreportalv1alpha2.GroupVersion.String(),
					Kind:       "Portal",
					Name:       "portal-1",
					UID:        "uid-portal",
					Controller: &isController,
				},
			},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("exactly one controller ownerReference"))
}

// Case 3: rejects DNSRecord ownerRef pointing to wrong Kind.
func TestDNSRecordWebhook_OwnerRef_RejectsWrongKind(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "")
	isController := true
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tRecordManual,
			Namespace: tNamespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: sreportalv1alpha2.GroupVersion.String(),
					Kind:       "Portal", // wrong Kind
					Name:       tDNSName,
					UID:        tDNSUID,
					Controller: &isController,
				},
			},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("controller ownerReference must be a DNS"))
}

// Case 3b: rejects DNSRecord ownerRef with wrong APIVersion.
func TestDNSRecordWebhook_OwnerRef_RejectsWrongAPIVersion(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t), "")
	isController := true
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tRecordManual,
			Namespace: tNamespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "sreportal.io/v1alpha1", // wrong APIVersion
					Kind:       "DNS",
					Name:       tDNSName,
					UID:        tDNSUID,
					Controller: &isController,
				},
			},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("controller ownerReference must be a DNS"))
}

// Case 4: rejects DNSRecord whose spec.portalRef differs from owner DNS spec.portalRef.
func TestDNSRecordWebhook_OwnerRef_RejectsMismatchedPortalRef(t *testing.T) {
	g := NewWithT(t)
	// DNS has portalRef "other", but DNSRecord has portalRef "main"
	dns := newDNS(tDNSName, tNamespace, tPortalOther)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordManual,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef(tDNSName, tDNSUID)},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain, // mismatch: DNS has "other"
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("does not match owner DNS.spec.portalRef"))
}

// Case 5: rejects update that mutates the controller ownerReference (UID change).
func TestDNSRecordWebhook_OwnerRef_RejectsOwnerRefMutation(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS(tDNSName, tNamespace, tPortalMain)
	// Also seed a second DNS so the new ownerRef lookup succeeds (we want to reach the immutability check).
	dns2 := newDNS("other-dns", tNamespace, tPortalMain)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns, dns2), "")
	isController := true
	oldOwnerRef := metav1.OwnerReference{
		APIVersion: sreportalv1alpha2.GroupVersion.String(),
		Kind:       "DNS",
		Name:       tDNSName,
		UID:        tDNSUID,
		Controller: &isController,
	}
	newOwnerRef := metav1.OwnerReference{
		APIVersion: sreportalv1alpha2.GroupVersion.String(),
		Kind:       "DNS",
		Name:       "other-dns",
		UID:        "other-uid-5678", // different UID
		Controller: &isController,
	}
	old := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{oldOwnerRef},
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
			OwnerReferences: []metav1.OwnerReference{newOwnerRef},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:    sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []sreportalv1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	_, err := v.ValidateUpdate(context.Background(), old, newR)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("controller ownerReference is immutable"))
}

// Case 6: accepts DNSRecord with a valid DNS ownerReference and matching portalRef.
func TestDNSRecordWebhook_OwnerRef_AcceptsValidOwnerRef(t *testing.T) {
	g := NewWithT(t)
	dns := newDNS(tDNSName, tNamespace, tPortalMain)
	v := webhookv1alpha2.NewDNSRecordCustomValidator(newFakeClient(t, dns), "")
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tRecordManual,
			Namespace:       tNamespace,
			OwnerReferences: []metav1.OwnerReference{validOwnerRef(tDNSName, tDNSUID)},
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
