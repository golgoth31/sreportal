---
title: Development
weight: 7
---

This page covers the development workflow for contributing to SRE Portal.

## Prerequisites

- Go 1.25+
- Docker
- `kubectl` with access to a Kubernetes cluster
- Node.js (for the web UI)
- [Buf CLI](https://buf.build/) (for protobuf codegen)

## Make Targets

### Build and Run

| Command | Description |
|---------|-------------|
| `make build` | Build the manager binary to `bin/manager` |
| `make run` | Run the controller locally using the current kubeconfig |
| `make docker-build IMG=<image>` | Build the container image |
| `make docker-push IMG=<image>` | Push the container image |

### Code Generation

| Command | Description |
|---------|-------------|
| `make manifests` | Generate CRDs, RBAC, and webhook manifests from kubebuilder markers |
| `make generate` | Generate `DeepCopy` methods (`zz_generated.deepcopy.go`) |
| `make proto` | Generate Go and TypeScript code from protobuf definitions (Buf) |
| `make proto-lint` | Lint protobuf files with Buf |

### Testing

| Command | Description |
|---------|-------------|
| `make test` | Run unit tests with envtest (in-memory K8s API + etcd) |
| `make test-e2e` | Run end-to-end tests on an isolated Kind cluster |
| `go test ./path/to/package -run TestName -v` | Run a single test |

### Linting

| Command | Description |
|---------|-------------|
| `make lint` | Run golangci-lint |
| `make lint-fix` | Auto-fix lint issues |

### Web UI

| Command | Description |
|---------|-------------|
| `make install-web` | Install npm dependencies (`npm install`) |
| `make build-web` | Build the Angular app for production |
| `npm test --prefix web` | Run web UI unit tests |

### Deployment

| Command | Description |
|---------|-------------|
| `make install` | Install CRDs into the cluster |
| `make deploy IMG=<image>` | Deploy the operator to the cluster |

## Testing

### Unit Tests

Unit tests use [Ginkgo v2](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) with envtest, which runs an in-memory Kubernetes API server and etcd.

Each test package has a `suite_test.go` that sets up the envtest environment in `BeforeSuite` and tears it down in `AfterSuite`.

Tests follow a BDD structure:

```go
var _ = Describe("Controller", func() {
    Context("when resource is created", func() {
        It("should reconcile", func() {
            Eventually(func(g Gomega) {
                // assertions
            }).Should(Succeed())
        })
    })
})
```

### End-to-End Tests

E2E tests run against a real Kind cluster and are guarded by the `e2e` build tag:

```bash
make test-e2e
```

## Adding New CRDs

Always use the kubebuilder CLI to scaffold new APIs and webhooks:

```bash
# Create a new API (controller + types)
kubebuilder create api --group sreportal --version v1alpha1 --kind <Kind>

# Create a webhook
kubebuilder create webhook --group sreportal --version v1alpha1 --kind <Kind> \
  --defaulting --programmatic-validation
```

After editing `*_types.go` files or kubebuilder markers, regenerate manifests:

```bash
make manifests generate
```

## Protobuf Codegen

Proto definitions live in `proto/sreportal/v1/`. After editing `.proto` files:

```bash
make proto
```

This generates:
- Go code in `internal/grpc/gen/`
- TypeScript code in `web/src/gen/`

## Critical Rules

**Never edit auto-generated files:**
- `config/crd/bases/*.yaml`
- `config/rbac/role.yaml`
- `config/webhook/manifests.yaml`
- `**/zz_generated.*.go`
- `internal/grpc/gen/*`
- `web/src/gen/*`
- `PROJECT`

**Never remove scaffold markers:**
```go
// +kubebuilder:scaffold:*
```

These markers are used by kubebuilder to insert new code when scaffolding additional APIs.
