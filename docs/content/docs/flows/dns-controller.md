---
title: DNS Controller Flow
weight: 2
---

The DNS controller reconciles `DNS` Custom Resources, which contain **manually defined** DNS entry groups. It aggregates manual entries, resolves DNS, and projects the result into the ReadStore.

## Overview

```mermaid
flowchart TD
    DNS["DNS CR\n(spec.groups: manual entries)"] --> Chain["DNS Controller\n(Chain of Responsibility)"]
    Chain --> Status["DNS CR Status\n(status.groups with syncStatus)"]
    Chain --> Store["FQDNStore\n(in-memory ReadStore)"]
    Store --> API["Connect gRPC API / MCP"]
```

## Trigger

The DNS controller is **watch-based**: it triggers whenever a `DNS` CR is created, updated, or deleted. It requeues every 5 minutes for periodic DNS resolution refresh.

## Chain of Responsibility

```mermaid
flowchart TD
    Start([Reconcile]) --> H1
    H1["① CollectManualEntries\nExtract groups from DNS.spec.groups"] --> H2
    H2["② AggregateFQDNs\nConvert to FQDNGroupStatus\nSet source = manual, sort by name"] --> H3
    H3["③ ResolveDNS\nParallel DNS lookup per FQDN\n(10 workers, 5s timeout)\nSkipped if disableDNSCheck=true\nSkipped for remote-source groups"] --> H4
    H4["④ UpdateStatus\nPatch DNS.status.groups\nSet Ready condition\nProject to FQDNWriter"] --> H5
    H5["⑤ ReconcileManualComponents\nCreate/update/delete Component CR\nfrom DNS CR annotations"] --> Done([Done])
```

### Step 1 — CollectManualEntries

Extracts DNS groups from `DNS.spec.groups`. Each group contains a name and a list of FQDNs with record type, targets, and optional labels.

### Step 2 — AggregateFQDNs

Converts spec groups to `FQDNGroupStatus` objects with `source: manual`. Groups are sorted alphabetically by name for deterministic output.

### Step 3 — ResolveDNS

For each FQDN in the aggregated groups (except those with source `remote`):

| DNS Result | SyncStatus |
|---|---|
| Resolved IPs match targets | `sync` |
| Resolved IPs differ from targets | `notFound` |
| DNS lookup error | `error` |
| Lookup exceeds 5s | `timeout` |

Resolution uses up to 10 concurrent goroutines. This step is skipped entirely when `reconciliation.disableDNSCheck: true` is set in the operator config.

### Step 4 — UpdateStatus

1. Patches `DNS.status.groups` with the resolved data
2. Sets `DNS.status.lastReconcileTime`
3. Sets a `Ready` condition (True on success, False with reason on error)
4. **Projects to ReadStore**: converts status groups to `[]FQDNView` and writes via `fqdnWriter.Replace(key, views)`

On CR deletion, the controller removes the corresponding key from the FQDNWriter.

### Step 5 — ReconcileManualComponents

If the DNS CR has a `sreportal.io/component` annotation:

1. **Skip** if the portal's status page feature is disabled
2. **Create or update** a Component CR from the annotation metadata
3. **Sync metadata** (`displayName`, `group`, `description`, `link`) but **never overwrite `spec.status`**
4. **Label** the component with `sreportal.io/managed-by: dns-controller`

If the annotation is **removed**, any previously created `dns-controller`-managed component for this portal is deleted.

See the [Annotations]({{< relref "annotations" >}}) page for the full list of `sreportal.io/component-*` annotations.

## ReadStore Projection

Each `FQDNView` from a DNS CR has:
- `Source: "manual"` (distinguishes from external-dns sources)
- `PortalName`: from the DNS CR's `spec.portalRef`
- `Groups`: from the spec group name
- `SyncStatus`: from DNS resolution
