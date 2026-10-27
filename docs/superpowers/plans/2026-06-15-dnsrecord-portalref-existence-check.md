# DNSRecord portalRef Existence Check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject standalone DNSRecord v1alpha2 CRs (no controller `ownerReference` to a DNS) whose `spec.portalRef` names a Portal that doesn't exist in the record's namespace.

**Architecture:** Extend `DNSRecordCustomValidator.validateOwnerRef`'s `!hasRef` branch (currently `return nil`) to call a new `validatePortalRefExists` helper that does a `client.Get` on `sreportalv1alpha1.Portal{Name: spec.portalRef, Namespace: r.Namespace}`, mirroring the existing v1alpha1 DNS webhook's `validatePortalRef` (`internal/webhook/v1alpha1/dns_webhook.go:94-109`). Owned records (`hasRef == true`) are unaffected — their `portalRef` is already pinned to, and validated via, their owner DNS.

**Tech Stack:** Go 1.26, controller-runtime fake client (`sigs.k8s.io/controller-runtime/pkg/client/fake`), Go `testing` + Gomega (`. "github.com/onsi/gomega"`).

**Reference:** Design doc `docs/superpowers/specs/2026-06-15-dnsrecord-portal-existence-validation-design.md`.

---

### Task 1: Portal-existence check for standalone DNSRecords

**Files:**
- Modify: `internal/webhook/v1alpha2/dnsrecord_webhook.go`
- Modify: `internal/webhook/v1alpha2/dnsrecord_webhook_test.go`

- [ ] **Step 1: Register the v1alpha1 scheme and add a `newPortal()` fixture in the test file**

In `internal/webhook/v1alpha2/dnsrecord_webhook_test.go`, update the import block (currently lines 19-35):

```go
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
```

(Only change: add the `sreportalv1alpha1` import line.)

Then update `newFakeClient` (currently lines 48-55):

```go
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
```

Then, immediately after `newDNS()` (currently lines 58-68) and before `validOwnerRef` (currently line 70), add a new fixture function:

```go
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
```

- [ ] **Step 2: Update `TestDNSRecordWebhook_ManualWithoutOwnerRefAccepted` to seed the Portal fixture**

Replace the existing test (currently lines 418-431):

```go
func TestDNSRecordWebhook_ManualWithoutOwnerRefAccepted(t *testing.T) {
	g := NewWithT(t)
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
	g.Expect(err).NotTo(HaveOccurred())
}
```

with:

```go
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
```

(Only change: seed `newPortal()` into the fake client.)

- [ ] **Step 3: Write the new failing test**

Immediately after the test from Step 2 (i.e. right before `TestDNSRecordWebhook_ManualWithDanglingOwnerRefRejected`, currently starting at line 433), insert:

```go
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
```

- [ ] **Step 4: Run the new test to verify it fails**

Run: `go test ./internal/webhook/v1alpha2/... -run TestDNSRecordWebhook_ManualWithoutOwnerRefRejectedWhenPortalMissing -v`

Expected: **FAIL** — `err` is `nil` (current `validateOwnerRef` returns `nil` for `!hasRef`), so `g.Expect(err).To(HaveOccurred())` fails.

- [ ] **Step 5: Implement `validatePortalRefExists` and wire it into `validateOwnerRef`**

In `internal/webhook/v1alpha2/dnsrecord_webhook.go`, update the import block (currently lines 19-32):

```go
import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/log"
)
```

(Only change: add the `sreportalv1alpha1` import line.)

Then, in `validateOwnerRef` (currently lines 156-158), replace:

```go
	if !hasRef {
		return nil
	}
```

with:

```go
	if !hasRef {
		return v.validatePortalRefExists(ctx, r)
	}
```

Then, immediately after `validateOwnerRef` (currently ends at line 178) and before `dnsControllerOwnerRefs` (currently starts at line 180), add the new helper:

```go

// validatePortalRefExists checks that spec.portalRef names a Portal in the
// DNSRecord's own namespace. Only called for records with no controller
// ownerReference to a DNS — owned records' portalRef is pinned to (and was
// validated via) their owner DNS.
func (v *DNSRecordCustomValidator) validatePortalRefExists(ctx context.Context, r *sreportalv1alpha2.DNSRecord) error {
	if r.Spec.PortalRef == "" {
		return fmt.Errorf("spec.portalRef is required")
	}

	var portal sreportalv1alpha1.Portal
	key := types.NamespacedName{Name: r.Spec.PortalRef, Namespace: r.Namespace}
	if err := v.client.Get(ctx, key, &portal); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("referenced portal %q not found in namespace %q", r.Spec.PortalRef, r.Namespace)
		}
		return fmt.Errorf("failed to check portal reference: %w", err)
	}

	return nil
}
```

- [ ] **Step 6: Run the full webhook test package to verify everything passes**

Run: `go test ./internal/webhook/v1alpha2/... -v`

Expected: **PASS** — all tests including `TestDNSRecordWebhook_ManualWithoutOwnerRefAccepted` and the new `TestDNSRecordWebhook_ManualWithoutOwnerRefRejectedWhenPortalMissing`.

- [ ] **Step 7: Lint**

Run: `make lint`

Expected: no new issues in `internal/webhook/v1alpha2/`.

- [ ] **Step 8: Commit**

```bash
git add internal/webhook/v1alpha2/dnsrecord_webhook.go internal/webhook/v1alpha2/dnsrecord_webhook_test.go
git commit -m "$(cat <<'EOF'
fix(webhook): reject standalone DNSRecords with unknown portalRef

A DNSRecord with origin=manual and no controller ownerReference to a
DNS previously accepted any spec.portalRef without checking that the
referenced Portal exists. validateOwnerRef's !hasRef branch now calls
validatePortalRefExists, mirroring the v1alpha1 DNS hub webhook's
validatePortalRef. Owned records are unaffected: their portalRef is
already pinned to, and validated via, their owner DNS.
EOF
)"
```

---

## Post-plan verification (not part of Task 1, run after merge readiness check)

- `make test` — full suite (envtest), to confirm no cross-package regressions.
- `make manifests generate` — not required (no `*_types.go` changes in this plan).
