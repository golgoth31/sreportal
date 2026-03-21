---
title: Configuration
weight: 3
---

The operator is configured through a ConfigMap that is mounted as a file at `/etc/sreportal/config.yaml`.

## ConfigMap

Create the ConfigMap in the operator namespace:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: sreportal-config
  namespace: sreportal-system
data:
  config.yaml: |
    sources:
      service:
        enabled: true
      ingress:
        enabled: true
    groupMapping:
      defaultGroup: "Services"
    reconciliation:
      interval: 5m
      retryOnError: 30s
```

## Sources

Each source type discovers DNS records from a different kind of Kubernetes resource.

### Service

Discovers DNS names from Kubernetes Services that have the `external-dns.alpha.kubernetes.io/hostname` annotation.

```yaml
sources:
  service:
    enabled: true
    namespace: ""              # Empty = all namespaces
    annotationFilter: ""       # Label selector syntax to filter services
    labelFilter: ""            # Label selector to filter services
    serviceTypeFilter:         # Service types to include
      - LoadBalancer
      - ClusterIP
    publishInternal: true      # Publish ClusterIP for ClusterIP services
    publishHostIP: false       # Publish host IP for NodePort services
    fqdnTemplate: ""           # Go template for FQDN generation
    combineFqdnAndAnnotation: false
    ignoreHostnameAnnotation: false
    resolveLoadBalancerHostname: false
```

### Ingress

Discovers DNS names from Kubernetes Ingress resources.

```yaml
sources:
  ingress:
    enabled: true
    namespace: ""              # Empty = all namespaces
    annotationFilter: ""       # Label selector syntax
    labelFilter: ""            # Label selector
    ingressClassNames: []      # Empty = all ingress classes
    fqdnTemplate: ""           # Go template for FQDN generation
    combineFqdnAndAnnotation: false
    ignoreHostnameAnnotation: false
    ignoreIngressTLSSpec: false
    ignoreIngressRulesSpec: false
    resolveLoadBalancerHostname: false
```

### DNSEndpoint

Reads external-dns `DNSEndpoint` CRDs directly. Requires external-dns CRDs to be installed in the cluster.

```yaml
sources:
  dnsEndpoint:
    enabled: false             # Disabled by default
    namespace: ""              # Empty = all namespaces
```

### Istio Gateway

Discovers DNS names from Istio Gateway resources. Requires Istio CRDs.

```yaml
sources:
  istioGateway:
    enabled: true              # Enabled by default
    namespace: ""              # Empty = all namespaces
    annotationFilter: ""
    fqdnTemplate: ""
    combineFqdnAndAnnotation: false
    ignoreHostnameAnnotation: false
```

### Istio VirtualService

Discovers DNS names from Istio VirtualService resources. Requires Istio CRDs.

```yaml
sources:
  istioVirtualService:
    enabled: false             # Disabled by default
    namespace: ""              # Empty = all namespaces
    annotationFilter: ""
    fqdnTemplate: ""
    combineFqdnAndAnnotation: false
    ignoreHostnameAnnotation: false
```

### Gateway API HTTPRoute

Discovers DNS names from Gateway API HTTPRoute resources. Requires Gateway API CRDs (v1.0+).

```yaml
sources:
  gatewayHTTPRoute:
    enabled: false             # Disabled by default
    namespace: ""              # Empty = all namespaces
    annotationFilter: ""
    labelFilter: ""
    fqdnTemplate: ""
    combineFqdnAndAnnotation: false
    ignoreHostnameAnnotation: false
    gatewayName: ""            # Filter by parent Gateway name
    gatewayNamespace: ""       # Filter by parent Gateway namespace
    gatewayLabelFilter: ""     # Filter parent Gateways by label selector
```

### Gateway API GRPCRoute

Discovers DNS names from Gateway API GRPCRoute resources. Requires Gateway API CRDs (v1.0+).

```yaml
sources:
  gatewayGRPCRoute:
    enabled: false             # Disabled by default
    namespace: ""              # Empty = all namespaces
    annotationFilter: ""
    labelFilter: ""
    fqdnTemplate: ""
    combineFqdnAndAnnotation: false
    ignoreHostnameAnnotation: false
    gatewayName: ""
    gatewayNamespace: ""
    gatewayLabelFilter: ""
```

### Gateway API TLSRoute

Discovers DNS names from Gateway API TLSRoute resources. Requires Gateway API CRDs (v1alpha2).

```yaml
sources:
  gatewayTLSRoute:
    enabled: false             # Disabled by default
    namespace: ""              # Empty = all namespaces
    annotationFilter: ""
    labelFilter: ""
    fqdnTemplate: ""
    combineFqdnAndAnnotation: false
    ignoreHostnameAnnotation: false
    gatewayName: ""
    gatewayNamespace: ""
    gatewayLabelFilter: ""
```

### Gateway API TCPRoute

Discovers DNS names from Gateway API TCPRoute resources. Requires Gateway API CRDs (v1alpha2).

Note: TCPRoute specs do not contain hostnames. Use the `external-dns.alpha.kubernetes.io/hostname` annotation on the TCPRoute to specify hostnames.

```yaml
sources:
  gatewayTCPRoute:
    enabled: false             # Disabled by default
    namespace: ""              # Empty = all namespaces
    annotationFilter: ""
    labelFilter: ""
    fqdnTemplate: ""
    combineFqdnAndAnnotation: false
    ignoreHostnameAnnotation: false
    gatewayName: ""
    gatewayNamespace: ""
    gatewayLabelFilter: ""
```

### Gateway API UDPRoute

Discovers DNS names from Gateway API UDPRoute resources. Requires Gateway API CRDs (v1alpha2).

Note: UDPRoute specs do not contain hostnames. Use the `external-dns.alpha.kubernetes.io/hostname` annotation on the UDPRoute to specify hostnames.

```yaml
sources:
  gatewayUDPRoute:
    enabled: false             # Disabled by default
    namespace: ""              # Empty = all namespaces
    annotationFilter: ""
    labelFilter: ""
    fqdnTemplate: ""
    combineFqdnAndAnnotation: false
    ignoreHostnameAnnotation: false
    gatewayName: ""
    gatewayNamespace: ""
    gatewayLabelFilter: ""
```

### Priority

Controls which source wins when the same FQDN + record type is discovered by multiple sources. Sources listed first take precedence over sources listed later.

When `priority` is empty or omitted, targets from all sources are merged together (default behaviour).

```yaml
sources:
  priority:
    - dnsendpoint
    - gateway-httproute
    - gateway-grpcroute
    - gateway-tlsroute
    - gateway-tcproute
    - gateway-udproute
    - ingress
    - service
    - istio-gateway
    - istio-virtualservice
```

Valid values correspond to the source types: `dnsendpoint`, `ingress`, `service`, `istio-gateway`, `istio-virtualservice`, `gateway-httproute`, `gateway-grpcroute`, `gateway-tlsroute`, `gateway-tcproute`, `gateway-udproute`.

## Group Mapping

Controls how discovered FQDNs are organized into groups in the web dashboard.

```yaml
groupMapping:
  defaultGroup: "Services"     # Fallback group name
  labelKey: ""                 # Endpoint label key for grouping
  byNamespace:                 # Map namespaces to group names
    production: "Production"
    staging: "Staging"
    monitoring: "Observability"
    default: "Development"
```

The group for each endpoint is resolved in priority order:

1. `sreportal.io/groups` annotation on the source resource (highest priority, supports comma-separated values)
2. Endpoint label matching `labelKey`
3. Namespace mapping via `byNamespace`
4. `defaultGroup` fallback

See [Annotations](../annotations) for details on annotation-based grouping.

## Reconciliation

Controls how often the source controller polls for DNS changes and optional features applied during each reconciliation cycle.

```yaml
reconciliation:
  interval: 5m             # How often to poll sources (default: 5m)
  retryOnError: 30s        # Retry delay after an error (default: 30s)
  disableDNSCheck: false   # Disable live DNS resolution (default: false)
```

### `disableDNSCheck`

When `false` (default), every FQDN is resolved against DNS during each reconciliation and a `syncStatus` field is populated on the FQDN status (`sync`, `notsync`, or `notavailable`).

Set to `true` to skip the resolution step entirely. This is useful when:

- The operator runs in a network environment without outbound DNS access
- DNS resolution latency is unacceptable for large FQDN counts
- The `syncStatus` field is not needed in the web UI

When disabled, `syncStatus` will be empty on all FQDNs.

## Release

Controls the Release CRD feature for tracking deployments, rollbacks, and other release events.

```yaml
release:
  ttl: 720h                 # How long Release CRs are kept (default: 720h = 30 days)
  namespace: ""             # Namespace for Release CRs (defaults to operator namespace)
```

| Field | Default | Description |
|-------|---------|-------------|
| `ttl` | `720h` (30 days) | Release CRs older than this are automatically deleted by the Release controller |
| `namespace` | _(operator namespace)_ | Namespace where Release CRs are stored |

The Release controller re-checks each CR every 12 hours. When a CR's day is older than the TTL, the controller deletes it automatically.

## Full Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: sreportal-config
  namespace: sreportal-system
data:
  config.yaml: |
    sources:
      service:
        enabled: true
        namespace: ""
        annotationFilter: ""
        serviceTypeFilter:
          - LoadBalancer
          - ClusterIP
        publishInternal: true
        publishHostIP: false
      ingress:
        enabled: true
        namespace: ""
        ingressClassNames: []
      dnsEndpoint:
        enabled: false
        namespace: ""
      istioGateway:
        enabled: true
        namespace: ""
      istioVirtualService:
        enabled: false
        namespace: ""
      gatewayHTTPRoute:
        enabled: false
        namespace: ""
        gatewayName: ""
        gatewayNamespace: ""
      gatewayGRPCRoute:
        enabled: false
        namespace: ""
      gatewayTLSRoute:
        enabled: false
        namespace: ""
      gatewayTCPRoute:
        enabled: false
        namespace: ""
      gatewayUDPRoute:
        enabled: false
        namespace: ""
      priority:
        - dnsendpoint
        - gateway-httproute
        - gateway-grpcroute
        - gateway-tlsroute
        - gateway-tcproute
        - gateway-udproute
        - ingress
        - service
        - istio-gateway
        - istio-virtualservice
    groupMapping:
      defaultGroup: "Services"
      labelKey: "sreportal.io/group"
      byNamespace:
        production: "Production"
        staging: "Staging"
        monitoring: "Observability"
        default: "Development"
    reconciliation:
      interval: 30s
      retryOnError: 10s
      disableDNSCheck: false
    release:
      ttl: 720h
      namespace: ""
```
