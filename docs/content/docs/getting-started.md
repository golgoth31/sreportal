---
title: Getting Started
weight: 1
---

This guide walks you through installing SRE Portal and creating your first portal with DNS discovery.

## Prerequisites

- A Kubernetes cluster (v1.28+)
- `kubectl` configured to access the cluster
- (Optional) Go 1.25+ and Docker for building from source

## Install CRDs

Install the Custom Resource Definitions into the cluster:

```bash
make install
```

This applies the generated CRD manifests from `config/crd/bases/`.

## Deploy the Operator

### Using Kustomize

```bash
kubectl apply -k config/default
```

### Building from Source

Build and push the container image, then deploy:

```bash
make docker-build IMG=<registry>/<image>:<tag>
make docker-push IMG=<registry>/<image>:<tag>
make deploy IMG=<registry>/<image>:<tag>
```

### Verify

```bash
kubectl get pods -n sreportal-system
```

You should see the `sreportal-controller-manager` pod running.

## Quick Start

Once the operator is running, a default Portal named `main` is created automatically. You can start discovering DNS records right away.

### 1. Create a Portal

The operator creates a `main` portal on startup. To add another portal:

```yaml
apiVersion: sreportal.io/v1alpha1
kind: Portal
metadata:
  name: production
  namespace: sreportal-system
spec:
  title: "Production Services"
  subPath: "production"
```

### 2. Create a DNS Resource

A DNS resource links manual DNS entries to a portal:

```yaml
apiVersion: sreportal.io/v1alpha1
kind: DNS
metadata:
  name: dns-sample
  namespace: default
spec:
  portalRef: main
  groups:
    - name: APIs
      description: Backend API services
      entries:
        - fqdn: api.example.com
          description: Main API endpoint
        - fqdn: graphql.example.com
          description: GraphQL API
    - name: Monitoring
      description: Observability tools
      entries:
        - fqdn: grafana.example.com
          description: Grafana dashboards
```

### 3. Annotate Services for Auto-Discovery

The operator discovers DNS records from Kubernetes resources that have the `external-dns.alpha.kubernetes.io/hostname` annotation:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-web-app
  namespace: default
  annotations:
    external-dns.alpha.kubernetes.io/hostname: "myapp.example.com"
spec:
  type: ClusterIP
  ports:
    - port: 80
      targetPort: 8080
  selector:
    app: my-web-app
```

By default, discovered endpoints are routed to the `main` portal. Use the `sreportal.io/portal` annotation to route to a different portal (see [Annotations](../annotations)).

### 4. (Optional) Create a Remote Portal

A remote portal fetches DNS data from another SRE Portal instance instead of the local cluster. Set `spec.remote.url` to the base URL of the remote instance:

```yaml
apiVersion: sreportal.io/v1alpha1
kind: Portal
metadata:
  name: remote-cluster
  namespace: sreportal-system
spec:
  title: "Remote Cluster"
  subPath: "remote-cluster"
  remote:
    url: "https://sreportal.other-cluster.example.com"
    portal: "main"   # optional: defaults to the main portal of the remote instance
```

The operator syncs with the remote portal every 5 minutes. Fetched FQDNs appear with source `remote` and sync status is tracked in `status.remoteSync`.

#### TLS Configuration

You can configure TLS settings for remote portal connections via `spec.remote.tls`:

```yaml
# Self-signed certificate (skip verification)
spec:
  remote:
    url: "https://sreportal.dev.example.com"
    tls:
      insecureSkipVerify: true
---
# Custom CA + mTLS client certificate
spec:
  remote:
    url: "https://sreportal.corp.example.com"
    tls:
      caSecretRef:
        name: remote-portal-ca          # Secret with "ca.crt" key
      certSecretRef:
        name: remote-portal-client-cert # Secret with "tls.crt" and "tls.key" keys
```

The referenced Secrets must exist in the same namespace as the Portal resource.

> **Note:** `spec.remote` cannot be set on the `main` portal (`spec.main: true`).

### 5. Access the Web UI

The web dashboard is served by the operator on port 8082. Forward the port to your local machine:

```bash
kubectl port-forward -n sreportal-system svc/sreportal-controller-manager 8082:8082
```

Open [http://localhost:8082](http://localhost:8082) in your browser. The default route redirects to `/main/links`, showing all FQDNs for the main portal.

## Running Locally

For development, you can run the operator against your current kubeconfig:

```bash
make run
```

This starts the controller, gRPC API, and web server locally. See [Development](../development) for more details.

## Next Steps

- [Architecture](../architecture) -- understand CRD relationships and controller patterns
- [Configuration](../configuration) -- customize sources, grouping rules, and timing
- [Annotations](../annotations) -- route endpoints to portals and assign groups
