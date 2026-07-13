---
title: Configuration
weight: 3
---

DNS discovery configuration lives in **two places** today:

1. **The `DNS` custom resource (`sreportal.io/v1alpha2`)** — the primary, per-portal configuration surface. Each `DNS` CR configures which sources are enabled, their filters, group mapping, and reconciliation timing for the portal it references (`spec.portalRef`). This is what you edit day to day.
2. **The operator ConfigMap** (mounted at `/etc/sreportal/config.yaml`) — operator-wide settings that are not portal-specific: the global source-collector tick interval, authentication, the Release feature, and Slack emoji resolution. Its old `sources` / `groupMapping` keys still exist for one-time migration only (see [Legacy ConfigMap keys](#legacy-configmap-keys)).

## The DNS Custom Resource

```yaml
apiVersion: sreportal.io/v1alpha2
kind: DNS
metadata:
  name: main
  namespace: sreportal-system
spec:
  portalRef: main
  sources:
    ingress:
      enabled: true
      annotationFilter: "external-dns.alpha.kubernetes.io/hostname"
    service:
      enabled: true
      annotationFilter: "external-dns.alpha.kubernetes.io/hostname"
      serviceTypeFilter: [LoadBalancer]
    priority: [ingress, service]
  groupMapping:
    defaultGroup: Services
    labelKey: sreportal.io/group
    byNamespace:
      monitoring: Monitoring
      default: Development
  reconciliation:
    interval: 5m
    retryOnError: 30s
    disableDNSCheck: false
```

`spec.portalRef` is **required and immutable**. Multiple `DNS` CRs may reference the same Portal (N:1) — a common pattern for splitting discovery config per team. Each `DNS` CR produces its own set of `DNSRecord` CRs (named `{dns-name}-{sourceType}`).

### `spec.defaults`

Fallback `namespace` / `labelFilter` applied to every source in this CR when the source's own field is empty:

```yaml
spec:
  defaults:
    namespace: ""        # empty = all namespaces
    labelFilter: ""       # label selector syntax
```

### `spec.sources`

Each source type discovers endpoints from a different kind of Kubernetes resource. Every source (except `dnsEndpoint` and `crossplaneScalewayRecord`) shares a common set of fields:

| Field | Description |
|---|---|
| `enabled` | Turns the source on for this DNS CR (default `false`) |
| `namespace` | Restrict to a namespace; empty = all namespaces (falls back to `spec.defaults.namespace`) |
| `annotationFilter` | Label-selector-syntax filter on resource annotations |
| `labelFilter` | Label selector filter (falls back to `spec.defaults.labelFilter`) |
| `fqdnTemplate` | Go template for FQDN generation |
| `combineFqdnAndAnnotation` | Combine template-generated and annotation hostnames |
| `ignoreHostnameAnnotation` | Ignore the `external-dns.alpha.kubernetes.io/hostname` annotation |

#### `service`

```yaml
sources:
  service:
    enabled: true
    serviceTypeFilter: [LoadBalancer, ClusterIP]
    publishInternal: true      # publish ClusterIP for ClusterIP services
    publishHostIP: false       # publish host IP for NodePort services
```

#### `ingress`

```yaml
sources:
  ingress:
    enabled: true
    ingressClassNames: []      # empty = all ingress classes
```

#### `dnsEndpoint`

Reads external-dns `DNSEndpoint` CRDs directly. Only `enabled`, `namespace`, `labelFilter` apply (no `CommonSourceSpec`).

```yaml
sources:
  dnsEndpoint:
    enabled: false
    namespace: ""
    labelFilter: ""
```

#### `istioGateway` / `istioVirtualService`

Require Istio CRDs installed in the cluster.

```yaml
sources:
  istioGateway:
    enabled: true
  istioVirtualService:
    enabled: false
```

#### Gateway API routes: `gatewayHTTPRoute`, `gatewayGRPCRoute`, `gatewayTLSRoute`, `gatewayTCPRoute`, `gatewayUDPRoute`

Require Gateway API CRDs (`gatewayHTTPRoute`/`gatewayGRPCRoute` need v1.0+, the others v1alpha2). Each adds parent-Gateway filters on top of the common fields:

```yaml
sources:
  gatewayHTTPRoute:
    enabled: false
    gatewayName: ""            # filter by parent Gateway name
    gatewayNamespace: ""       # filter by parent Gateway namespace
    gatewayLabelFilter: ""     # filter parent Gateways by label selector
```

`gatewayTCPRoute` and `gatewayUDPRoute` specs carry no hostname — use the `external-dns.alpha.kubernetes.io/hostname` annotation on the route itself.

#### `crossplaneScalewayRecord`

Discovers DNS names from Crossplane Scaleway `Record` resources. Only `enabled`, `namespace`, `labelFilter`, `clusterScoped` apply.

```yaml
sources:
  crossplaneScalewayRecord:
    enabled: false
    clusterScoped: false
```

#### `priority`

Controls which source wins when the same FQDN is discovered by multiple sources within this DNS CR. Sources listed first take precedence; unlisted enabled sources rank lowest. The DNS webhook rejects a `priority` entry for a source that isn't `enabled` in the same CR.

```yaml
sources:
  priority:
    - dnsendpoint
    - ingress
    - service
    - istio-gateway
    - istio-virtualservice
    - gateway-httproute
    - gateway-grpcroute
    - gateway-tlsroute
    - gateway-tcproute
    - gateway-udproute
    - crossplane-scaleway-record
```

Deduplication happens at the FQDN-name level (not per record type): the winning source keeps every record type it produced for that name; the losing source drops all records for that name. See the [DNS Controller Flow]({{< relref "flows/dns-controller" >}}) for the exact algorithm.

### How collection and per-DNS filtering interact

Endpoint **collection** is cluster-wide and shared: a single background collector lists each enabled Kubernetes resource kind once per tick and caches the result in an in-memory `SourceEndpointStore` (see the [DNS Source Flow]({{< relref "flows/dns-source" >}})). The set of kinds actually watched, and the collection-time knobs (namespace scope, `annotationFilter`, `fqdnTemplate`, `ignoreHostnameAnnotation`, etc.), are the **union of every non-remote `DNS` CR's settings for that kind** — the most permissive value wins so no CR under-discovers.

Each `DNS` CR then reads from that shared store and applies its **own** `namespace` / `labelFilter` narrowing at read time. Practically: if any DNS CR in the cluster enables `service` cluster-wide, the collector watches all namespaces for Services; a second DNS CR can still restrict itself to `namespace: team-a` when it reads the store.

### `spec.groupMapping`

Controls how this DNS CR's discovered FQDNs are organized into groups in the web dashboard.

```yaml
groupMapping:
  defaultGroup: "Services"     # fallback group name (required, default "Services")
  labelKey: ""                 # endpoint label key for grouping
  byNamespace:                 # namespace -> group name
    production: "Production"
    staging: "Staging"
```

The group for each endpoint is resolved in priority order:

1. `sreportal.io/groups` annotation on the source resource (highest priority, comma-separated)
2. Endpoint label matching `labelKey`
3. Namespace mapping via `byNamespace`
4. `defaultGroup` fallback

See [Annotations](../annotations) for details on annotation-based grouping.

### `spec.reconciliation`

```yaml
reconciliation:
  interval: 5m             # DNS controller requeue interval (default 5m, floor 30s)
  retryOnError: 30s        # reserved field, not currently consumed by any controller
  disableDNSCheck: false   # skip live DNS resolution for this CR's records
```

`interval` paces the `DNS` controller's own reconcile loop (clamped to a 30s minimum). `disableDNSCheck` is read by the async DNS-resolution runnable (see below) for every `DNSRecord` governed by this `DNS` CR — when `true`, `syncStatus` is never populated for those records. `retryOnError` is accepted by the schema for forward compatibility but nothing currently reads it; the controller relies on controller-runtime's default error-requeue behavior instead.

## Manual DNS entries

There is no more "manual" mode on the `DNS` CR. To hand-author DNS entries, create a `DNSRecord` with `spec.origin: manual` directly:

```yaml
apiVersion: sreportal.io/v1alpha2
kind: DNSRecord
metadata:
  name: main-manual-apis
  namespace: default
spec:
  origin: manual
  portalRef: main
  entries:
    - fqdn: api.example.com
      group: APIs
      description: Main API endpoint
      recordType: A
    - fqdn: graphql.example.com
      group: APIs
      description: GraphQL API
```

A manual `DNSRecord` is reconciled by the same `DNSRecord` controller as auto-discovered records (group mapping and `disableDNSCheck` are still inherited from the `DNS` CR matching `spec.portalRef`). See the [DNSRecord Controller Flow]({{< relref "flows/dnsrecord" >}}).

`spec.origin` and `spec.portalRef` are immutable. The validating webhook restricts writes to `origin: auto` records to the operator's own ServiceAccount, so a human editing an auto-discovered `DNSRecord` is rejected at admission — any change is overwritten at the next DNS reconcile anyway.

## The operator ConfigMap

Mounted at `/etc/sreportal/config.yaml`. If the file is absent, built-in defaults are used.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: sreportal-config
  namespace: sreportal-system
data:
  config.yaml: |
    reconciliation:
      interval: 5m

    release:
      ttl: 720h
      types:
        - name: deployment
          color: "#3b82f6"
        - name: rollback
          color: "#f97316"

    auth:
      apiKey:
        enabled: false
        headerName: "X-API-Key"
      jwt:
        enabled: false
        issuers: []

    emoji:
      slack:
        enabled: false
        refreshInterval: 24h
```

### Fields actually in use

| Key | Used for |
|---|---|
| `reconciliation.interval` | Tick interval of the two cluster-wide background collectors: the source producer (`SourceReconciler`) and the Components reconciler. Default `5m`. |
| `release.ttl`, `release.namespace`, `release.types` | Release CRD feature — see below. |
| `auth.apiKey`, `auth.jwt` | Authentication for write endpoints (e.g. `AddRelease`). |
| `emoji.slack` | Custom emoji resolution from Slack for the web UI. |

### `release`

Controls the Release CRD feature for tracking deployments, rollbacks, and other release events.

| Field | Default | Description |
|-------|---------|-------------|
| `ttl` | `720h` (30 days) | Release CRs older than this are automatically deleted by the Release controller (checked every 12h) |
| `namespace` | _(operator namespace)_ | Namespace where Release CRs are stored |
| `types` | _(empty)_ | List of `{name, color}` entries. `name` allowlists `AddRelease` types; `color` is the CSS color sent to the web UI for the type badge. Empty means all types are accepted and the UI uses built-in default colors |

### `auth`

Each method has an `enabled` flag; multiple methods can coexist.

- `apiKey`: header-based API key. `headerName` defaults to `X-API-Key`. The actual key value is read from the `HEADER_API_KEY` environment variable, never from the ConfigMap.
- `jwt`: Bearer token validation against one or more `issuers` (`issuerURL`, `jwksURL`, optional `audience` / `requiredClaims`). At least one issuer is required when `jwt.enabled: true`.

### `emoji.slack`

Fetches custom emoji from Slack for rendering in the web UI. `refreshInterval` defaults to `24h`. The Slack API token is read from the `SLACK_API_TOKEN` environment variable.

## Legacy ConfigMap keys

The ConfigMap schema still accepts `sources` and `groupMapping` keys in the exact shape used before the `v1alpha2` DNS API existed, but **the operator no longer reads them on every reconcile**. They are consumed exactly once, the first time a Portal's main `DNS` CR is created (or upgraded from `v1alpha1`):

- if the ConfigMap carries a legacy `sources` block, it is translated into `DNS.spec.sources` / `spec.groupMapping` / `spec.reconciliation` and written onto the newly created (or freshly-converted, still-empty) `DNS` CR;
- the CR is then stamped with the `sreportal.io/sources-migrated: "true"` annotation;
- from that point on the `DNS` CR is the source of truth — the handler never looks at the ConfigMap for that portal again, even if you edit the ConfigMap afterwards.

If you are configuring a fresh install, skip the legacy keys entirely and configure `spec.sources` / `spec.groupMapping` / `spec.reconciliation` directly on the `DNS` CR — see [The DNS Custom Resource](#the-dns-custom-resource) above. A few advanced legacy knobs (`resolveLoadBalancerHostname`, `ignoreIngressTlsSpec`, `ignoreIngressRulesSpec`) have no `v1alpha2` equivalent and are silently dropped by the migration.

## DNS resolution (`syncStatus`)

Live DNS resolution is **not** part of either the `DNS` or `DNSRecord` reconcile loop. It runs in a separate background runnable that:

- resolves each `DNSRecord`'s endpoints on a per-FQDN schedule jittered across a 24h interval (so checks spread out instead of firing in bursts), polled every minute;
- skips a `DNSRecord` entirely when the governing `DNS` CR has `spec.reconciliation.disableDNSCheck: true`;
- can be forced immediately for a record right after its spec changes (debounced ~5s), so a newly added FQDN gets an initial status quickly instead of waiting up to 24h;
- writes `sync` / `notsync` / `notavailable` onto `DNSRecord.status.endpoints[].syncStatus`, which re-triggers the `DNSRecord` controller to re-project the new status into the read store.

See [DNSRecord Controller Flow]({{< relref "flows/dnsrecord" >}}) for details.
