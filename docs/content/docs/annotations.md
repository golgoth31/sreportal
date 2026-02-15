---
title: Annotations
weight: 4
---

SRE Portal uses two annotations on Kubernetes resources to control how discovered endpoints are routed and grouped.

## `sreportal.io/portal`

Routes endpoints from a resource to a specific portal. When this annotation is absent, endpoints are routed to the default `main` portal.

If the annotation references a portal that does not exist, the endpoint falls back to the main portal.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: api-server
  annotations:
    external-dns.alpha.kubernetes.io/hostname: "api.example.com"
    sreportal.io/portal: "production"
spec:
  type: LoadBalancer
  ports:
    - port: 443
  selector:
    app: api-server
```

## `sreportal.io/group`

Assigns endpoints from a resource to a custom group in the web dashboard. This annotation has the highest priority in the group resolution chain.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    external-dns.alpha.kubernetes.io/hostname: "nginx.example.com"
    sreportal.io/group: "Applications"
spec:
  type: LoadBalancer
  ports:
    - port: 80
  selector:
    app: nginx
```

## How Enrichment Works

The source controller enriches discovered endpoints with annotation values from the original Kubernetes resource:

1. External-dns sources produce endpoints with a resource label (`kind/namespace/name`)
2. The source controller looks up the original resource via the Kubernetes API
3. Any `sreportal.io/*` annotations on the resource are copied to the endpoint labels
4. These labels are then used for portal routing and group assignment

## Group Resolution Priority

When determining which group an endpoint belongs to, the operator checks these rules in order (first match wins):

| Priority | Source | Description |
|----------|--------|-------------|
| 1 | `sreportal.io/group` annotation | Annotation on the K8s resource |
| 2 | `labelKey` config | Endpoint label matching the configured `groupMapping.labelKey` |
| 3 | `byNamespace` config | Namespace-to-group mapping from `groupMapping.byNamespace` |
| 4 | `defaultGroup` config | Fallback from `groupMapping.defaultGroup` (default: `"Services"`) |

## Examples

### Service with Both Annotations

```yaml
apiVersion: v1
kind: Service
metadata:
  name: grafana
  namespace: monitoring
  annotations:
    external-dns.alpha.kubernetes.io/hostname: "grafana.example.com"
    sreportal.io/portal: "production"
    sreportal.io/group: "Observability"
spec:
  type: ClusterIP
  ports:
    - port: 3000
  selector:
    app: grafana
```

This service is discovered with the FQDN `grafana.example.com`, routed to the `production` portal, and placed in the `Observability` group.

### Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: web-app
  annotations:
    sreportal.io/portal: "production"
    sreportal.io/group: "Applications"
spec:
  rules:
    - host: app.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web-app
                port:
                  number: 80
```

### Istio Gateway

```yaml
apiVersion: networking.istio.io/v1
kind: Gateway
metadata:
  name: api-gateway
  annotations:
    sreportal.io/portal: "production"
    sreportal.io/group: "APIs"
spec:
  selector:
    istio: ingressgateway
  servers:
    - port:
        number: 443
        name: https
        protocol: HTTPS
      hosts:
        - "api.example.com"
```
