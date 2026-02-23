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
    enabled: false             # Disabled by default
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

Controls how often the source controller polls for DNS changes.

```yaml
reconciliation:
  interval: 5m       # How often to poll sources (default: 5m)
  retryOnError: 30s  # Retry delay after an error (default: 30s)
```

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
        enabled: false
        namespace: ""
      istioVirtualService:
        enabled: false
        namespace: ""
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
```
