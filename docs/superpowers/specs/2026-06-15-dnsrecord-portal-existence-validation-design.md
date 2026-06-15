# DNSRecord — reject standalone records with a non-existent portalRef

- **Date**: 2026-06-15
- **Status**: Design — pending implementation plan
- **Scope**: `internal/webhook/v1alpha2/dnsrecord_webhook.go` and its test suite
- **Out of scope**: DNS v1alpha2 webhook (already delegates Portal-existence checks to the v1alpha1 hub via conversion), v1alpha1 DNSRecord webhook (unrelated concern — guards the v1alpha2-spec annotation)
- **Related**: [[2026-05-15-dns-multi-cr-per-portal-design]] (introduced the ownerRef/portalRef invariants this design extends)

## 1. Motivation

A `Portal` can only "own" `DNS`/`DNSRecord` CRs in its own namespace (Portal is namespace-scoped; `DNS.spec.portalRef` is resolved against the DNS's own namespace by the v1alpha1 hub webhook, and an owned `DNSRecord.spec.portalRef` must equal its owner DNS's `portalRef`).

The one gap: a **standalone `DNSRecord`** (`origin=manual`, no controller `ownerReference` to a `DNS`) sets `spec.portalRef` as a free string with **no existence check at all**. Nothing today stops `spec.portalRef: does-not-exist`.

## 2. Design

Extend `DNSRecordCustomValidator.validateOwnerRef` (`internal/webhook/v1alpha2/dnsrecord_webhook.go`): the `!hasRef` branch (currently `return nil`) becomes a Portal-existence lookup, mirroring the existing v1alpha1 DNS webhook's `validatePortalRef` (`internal/webhook/v1alpha1/dns_webhook.go:94-109`).

```go
if !hasRef {
    return v.validatePortalRefExists(ctx, r)
}
```

```go
// validatePortalRefExists checks that spec.portalRef names a Portal in the
// DNSRecord's own namespace. Only called for records with no controller
// ownerReference to a DNS — owned records' portalRef is pinned to (and
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

New import: `sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"` (Portal has no v1alpha2 type; `apierrors` and `types` are already imported).

### Why gate on `!hasRef`

Owned records (`hasRef == true`) already require `spec.portalRef == owner.Spec.PortalRef` (`dnsrecord_webhook.go:173-174`), and the owner DNS's `portalRef` was validated to exist when the DNS itself was created/updated. Re-checking Portal existence for every owned record would add a redundant `client.Get` on the `origin=auto` materialisation hot path without catching anything new. Gating on `!hasRef` keeps the change surgical and limits the blast radius on the existing test matrix to a single test.

### Create vs Update

The check runs through `validate()`, which both `ValidateCreate` and `ValidateUpdate` call — same as the v1alpha1 DNS hub webhook, which checks Portal existence on both create and update. `spec.portalRef` is immutable, so on update this only bites if the referenced Portal was deleted after the record was created (consistent with existing DNS behaviour: a dangling `portalRef` blocks further admission until resolved).

## 3. Tests (`internal/webhook/v1alpha2/dnsrecord_webhook_test.go`)

- `newFakeClient` registers `sreportalv1alpha1.AddToScheme` alongside the existing v1alpha2 scheme.
- New fixture `newPortal()` — minimal `sreportalv1alpha1.Portal{Name: tPortalMain, Namespace: tNamespace}`.
- `TestDNSRecordWebhook_ManualWithoutOwnerRefAccepted` — seed `newPortal()` so it remains accepted (currently uses an empty fake client).
- New `TestDNSRecordWebhook_ManualWithoutOwnerRefRejectedWhenPortalMissing` — empty fake client, `origin=manual`, no ownerRef, `portalRef=tPortalMain` → expect error containing `"referenced portal"` and `"not found"`.

No other existing test is affected: all other success-path tests carry a controller ownerRef (`hasRef == true`), and all other failure-path tests short-circuit earlier in `validate()` (origin/sourceType/entries checks, immutability checks, or `validateOwnerRef`'s ownerRef-shape checks) before reaching the `!hasRef` branch.

## 4. Open questions

None — scope is fully bounded by the existing webhook test matrix reviewed above.
