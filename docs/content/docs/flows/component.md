---
title: Component Flow
weight: 8
---

The Component controller computes the effective operational status of each platform component, taking active maintenances into account.

## Overview

```mermaid
flowchart TD
    Comp["Component CR\n(spec.status = operational)"] --> Chain["Component Controller\n(Chain of Responsibility)"]
    Maint["Maintenance CR\n(phase = in_progress)"] -->|"watch: enqueue\naffected components"| Chain
    Chain --> Status["Component CR Status\n(computedStatus)"]
    Chain --> Store["ComponentStore\n(in-memory ReadStore)"]
    Store --> API["Connect gRPC API / MCP"]
    API --> UI["Web UI\n(Status page)"]
```

## Trigger

**Watch-based**: triggers on create/update/delete of `Component` CRs. Also watches `Maintenance` CRs — when a maintenance changes, all components listed in `spec.components[]` are re-enqueued.

## Chain of Responsibility

```mermaid
flowchart TD
    Start([Reconcile]) --> H1
    H1["① ValidatePortalRef\nVerify Portal CR exists\n→ Ready=False if not found"] --> H2
    H2["② ComputeStatus\nRead MaintenanceReader for in_progress\nmaintenances targeting this component\n→ Override to affectedStatus or keep spec.status"] --> H3
    H3["③ UpdateStatus\nPatch status (computedStatus, conditions)\nRe-fetch for fresh resourceVersion\nAdd sreportal.io/portal label\nProject to ComponentWriter"] --> Done([Done])
```

### Step 1 — ValidatePortalRef

Looks up the Portal CR by `spec.portalRef`. If not found:
- Sets `Ready` condition to `False` with reason `PortalNotFound`
- Returns error (chain stops, reconciler requeues after 30s)

### Step 2 — ComputeStatus

Reads from the **MaintenanceReader** (in-memory ReadStore, not K8s API) to find `in_progress` maintenances for the same portal. Uses `MaintenanceView.AffectsComponent()` domain method.

```
if any in_progress maintenance targets this component:
    computedStatus = maintenance.spec.affectedStatus  (e.g. "maintenance")
else:
    computedStatus = component.spec.status  (e.g. "operational")
```

### Step 3 — UpdateStatus

1. Detects status transitions (`oldStatus != newStatus`) and updates `lastStatusChange`
2. Sets `Ready=True` condition via shared `statusutil.SetConditionAndPatch()`
3. Re-fetches the CR to get a fresh `resourceVersion` (avoids conflict with the status patch)
4. Adds the `sreportal.io/portal` label
5. Projects `ComponentView` to the `ComponentWriter`

## Domain Types

```
Component CR (spec.status)
     │
     ▼  ComputeStatusHandler (+ MaintenanceReader)
ComputedComponentStatus        (status.computedStatus in etcd)
     │
     ▼
ComponentView                  (ReadStore, in-memory)
     │
     ▼
proto ComponentResource        (on the wire)
```
