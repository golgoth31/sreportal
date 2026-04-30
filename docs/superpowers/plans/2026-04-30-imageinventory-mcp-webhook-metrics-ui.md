# ImageInventory — MCP Server, Webhook, Metrics, Colored Badges

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add MCP server at `/mcp/image`, an admission webhook validating `spec.portalRef`, Prometheus metrics for image scanning, and colored tag-type badges in the React UI.

**Architecture:** MCP server follows the existing `AlertsServer`/`DNSServer` pattern in `internal/mcp/`. Webhook follows the `ReleaseCustomValidator` pattern in `internal/webhook/v1alpha1/`. Metrics are registered in `internal/metrics/metrics.go` and emitted from the `ScanWorkloadsHandler`. UI badges use `variant="outline"` + custom `className` per tag type, following the netpol pattern.

**Tech Stack:** Go 1.26, mark3labs/mcp-go, controller-runtime webhook, prometheus/client_golang, React 19 + shadcn/ui Badge.

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/mcp/image_server.go` | ImageServer struct, `list_images` MCP tool |
| Modify | `internal/mcp/mcp_test.go` | Add ImageServer Describe block |
| Modify | `cmd/main.go` | Wire ImageServer + mount `/mcp/image` |
| Create | `internal/webhook/v1alpha1/imageinventory_webhook.go` | ValidateCreate/Update/Delete + kubebuilder marker |
| Create | `internal/webhook/v1alpha1/imageinventory_webhook_test.go` | Fake-client unit tests for validator |
| Modify | `internal/webhook/v1alpha1/webhook_suite_test.go` | Register imageinventory webhook in envtest suite |
| Modify | `cmd/main.go` | Register SetupImageInventoryWebhookWithManager |
| Modify | `internal/metrics/metrics.go` | Add `ImageImagesTotal`, `ImageInventoryScanTotal`, register |
| Modify | `internal/controller/imageinventory/chain/scan_workloads.go` | Emit metrics after scan |
| Modify | `web/src/features/image/ui/ImageCard.tsx` | Colored badge by tag type |

---

## Task 1: MCP ImageServer — failing test

**Files:**
- Modify: `internal/mcp/mcp_test.go`

- [ ] **Step 1: Add import for image readstore and add failing Describe block**

Add to the imports in `internal/mcp/mcp_test.go`:
```go
domainimage "github.com/golgoth31/sreportal/internal/domain/image"
imagestore "github.com/golgoth31/sreportal/internal/readstore/image"
```

Append inside the outer `Describe("MCP Server", ...)` block, after the `ReleasesServer` Describe:
```go
Describe("ImageServer", func() {
    var store *imagestore.Store

    BeforeEach(func() {
        store = imagestore.NewStore()
    })

    Describe("creation", func() {
        It("should create server with list_images tool registered", func() {
            s := NewImageServer(store)
            Expect(s).NotTo(BeNil())
            Expect(s.mcpServer).NotTo(BeNil())
            Expect(s.reader).NotTo(BeNil())
        })
    })

    Describe("handleListImages", func() {
        Context("with images in the store", func() {
            BeforeEach(func() {
                _ = store.ReplaceAll(ctx, "main", map[domainimage.WorkloadKey][]domainimage.ImageView{
                    {Kind: "Deployment", Namespace: "default", Name: "api"}: {
                        {
                            PortalRef:  "main",
                            Registry:   "docker.io",
                            Repository: "myorg/api",
                            Tag:        "v1.2.3",
                            TagType:    domainimage.TagTypeSemver,
                            Workloads: []domainimage.WorkloadRef{
                                {Kind: "Deployment", Namespace: "default", Name: "api", Container: "api"},
                            },
                        },
                    },
                    {Kind: "Deployment", Namespace: "default", Name: "worker"}: {
                        {
                            PortalRef:  "main",
                            Registry:   "ghcr.io",
                            Repository: "myorg/worker",
                            Tag:        "abc1234",
                            TagType:    domainimage.TagTypeCommit,
                            Workloads: []domainimage.WorkloadRef{
                                {Kind: "Deployment", Namespace: "default", Name: "worker", Container: "worker"},
                            },
                        },
                    },
                })
            })

            It("should return all images when no filters", func() {
                s := NewImageServer(store)
                req := newCallToolRequest("list_images", map[string]any{})

                result, err := s.handleListImages(ctx, req)

                Expect(err).NotTo(HaveOccurred())
                Expect(isErrorResult(result)).To(BeFalse())
                text := extractTextContent(result)
                Expect(text).To(ContainSubstring("Found 2 image(s)"))
                Expect(text).To(ContainSubstring("myorg/api"))
                Expect(text).To(ContainSubstring("myorg/worker"))
            })

            It("should filter by portal", func() {
                _ = store.ReplaceAll(ctx, "dev", map[domainimage.WorkloadKey][]domainimage.ImageView{
                    {Kind: "Deployment", Namespace: "dev", Name: "svc"}: {
                        {
                            PortalRef:  "dev",
                            Registry:   "docker.io",
                            Repository: "myorg/svc",
                            Tag:        "latest",
                            TagType:    domainimage.TagTypeLatest,
                            Workloads:  []domainimage.WorkloadRef{{Kind: "Deployment", Namespace: "dev", Name: "svc", Container: "svc"}},
                        },
                    },
                })
                s := NewImageServer(store)
                req := newCallToolRequest("list_images", map[string]any{"portal": "dev"})

                result, err := s.handleListImages(ctx, req)

                Expect(err).NotTo(HaveOccurred())
                text := extractTextContent(result)
                Expect(text).To(ContainSubstring("Found 1 image(s)"))
                Expect(text).To(ContainSubstring("myorg/svc"))
                Expect(text).NotTo(ContainSubstring("myorg/api"))
            })

            It("should filter by registry", func() {
                s := NewImageServer(store)
                req := newCallToolRequest("list_images", map[string]any{"registry": "ghcr.io"})

                result, err := s.handleListImages(ctx, req)

                Expect(err).NotTo(HaveOccurred())
                text := extractTextContent(result)
                Expect(text).To(ContainSubstring("Found 1 image(s)"))
                Expect(text).To(ContainSubstring("myorg/worker"))
                Expect(text).NotTo(ContainSubstring("myorg/api"))
            })

            It("should filter by tag_type", func() {
                s := NewImageServer(store)
                req := newCallToolRequest("list_images", map[string]any{"tag_type": "semver"})

                result, err := s.handleListImages(ctx, req)

                Expect(err).NotTo(HaveOccurred())
                text := extractTextContent(result)
                Expect(text).To(ContainSubstring("Found 1 image(s)"))
                Expect(text).To(ContainSubstring("myorg/api"))
            })

            It("should filter by search substring", func() {
                s := NewImageServer(store)
                req := newCallToolRequest("list_images", map[string]any{"search": "worker"})

                result, err := s.handleListImages(ctx, req)

                Expect(err).NotTo(HaveOccurred())
                text := extractTextContent(result)
                Expect(text).To(ContainSubstring("Found 1 image(s)"))
                Expect(text).To(ContainSubstring("myorg/worker"))
            })
        })

        Context("with empty store", func() {
            It("should return no images message", func() {
                s := NewImageServer(store)
                req := newCallToolRequest("list_images", map[string]any{})

                result, err := s.handleListImages(ctx, req)

                Expect(err).NotTo(HaveOccurred())
                text := extractTextContent(result)
                Expect(text).To(Equal("No images found matching the criteria."))
            })
        })
    })
})
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
cd /Users/davidsabatie/Projects/go/src/github.com/golgoth31/sreportal
go test ./internal/mcp/... -run "ImageServer" -v 2>&1 | head -30
```

Expected output: compilation error — `NewImageServer` undefined, `imagestore` import not found.

---

## Task 2: MCP ImageServer — implementation

**Files:**
- Create: `internal/mcp/image_server.go`

- [ ] **Step 1: Create the file**

```go
/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ImageServer wraps the MCP server for image inventory.
// Mount at /mcp/image for Streamable HTTP.
type ImageServer struct {
	mcpServer *server.MCPServer
	reader    domainimage.ImageReader
}

// NewImageServer creates a new MCP server instance for image inventory.
func NewImageServer(reader domainimage.ImageReader) *ImageServer {
	s := &ImageServer{reader: reader}

	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-image")
		logger.Info("client session registered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("image").Inc()
	})
	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-image")
		logger.Info("client session unregistered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("image").Dec()
	})
	hooks.AddAfterInitialize(func(ctx context.Context, _ any, message *mcp.InitializeRequest, _ *mcp.InitializeResult) {
		logger := log.FromContext(ctx).WithName("mcp-image")
		logger.Info("client initialized",
			"clientName", message.Params.ClientInfo.Name,
			"clientVersion", message.Params.ClientInfo.Version,
			"protocolVersion", message.Params.ProtocolVersion,
		)
	})

	s.mcpServer = server.NewMCPServer(
		"sreportal-image",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithHooks(hooks),
	)

	s.registerImageTools()

	return s
}

func (s *ImageServer) registerImageTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("list_images",
			mcp.WithDescription("List container images discovered by ImageInventory resources in the SRE Portal. "+
				"Returns images with their tag type (semver, commit, digest, latest), registry, repository, and the workloads using them."),
			mcp.WithString("portal",
				mcp.Description("Filter by portal name (portalRef)"),
			),
			mcp.WithString("search",
				mcp.Description("Search in repository name (substring match)"),
			),
			mcp.WithString("registry",
				mcp.Description("Filter by registry hostname (e.g. docker.io, ghcr.io)"),
			),
			mcp.WithString("tag_type",
				mcp.Description("Filter by tag type: semver, commit, digest, or latest"),
			),
		),
		withToolMetrics("image", "list_images", s.handleListImages),
	)
}

// ImageWorkloadResult represents a workload that uses an image.
type ImageWorkloadResult struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Container string `json:"container"`
}

// ImageResult represents a single image entry in the list results.
type ImageResult struct {
	Registry   string                `json:"registry"`
	Repository string                `json:"repository"`
	Tag        string                `json:"tag"`
	TagType    string                `json:"tag_type"`
	Workloads  []ImageWorkloadResult `json:"workloads"`
}

func (s *ImageServer) handleListImages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	views, err := s.reader.List(ctx, domainimage.ImageFilters{
		Portal:   request.GetString("portal", ""),
		Search:   request.GetString("search", ""),
		Registry: request.GetString("registry", ""),
		TagType:  request.GetString("tag_type", ""),
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list images: %v", err)), nil
	}

	if len(views) == 0 {
		return mcp.NewToolResultText("No images found matching the criteria."), nil
	}

	results := make([]ImageResult, 0, len(views))
	for _, v := range views {
		workloads := make([]ImageWorkloadResult, 0, len(v.Workloads))
		for _, w := range v.Workloads {
			workloads = append(workloads, ImageWorkloadResult{
				Kind:      w.Kind,
				Namespace: w.Namespace,
				Name:      w.Name,
				Container: w.Container,
			})
		}
		results = append(results, ImageResult{
			Registry:   v.Registry,
			Repository: v.Repository,
			Tag:        v.Tag,
			TagType:    string(v.TagType),
			Workloads:  workloads,
		})
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d image(s):\n\n%s", len(results), string(jsonBytes))), nil
}

// Handler returns an http.Handler for the MCP Streamable HTTP transport.
// Mount at /mcp/image.
func (s *ImageServer) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}
```

- [ ] **Step 2: Run the tests to confirm they pass**

```bash
cd /Users/davidsabatie/Projects/go/src/github.com/golgoth31/sreportal
go test ./internal/mcp/... -run "ImageServer" -v 2>&1 | tail -20
```

Expected: all ImageServer specs PASS.

- [ ] **Step 3: Run the full MCP test suite to check for regressions**

```bash
go test ./internal/mcp/... -v 2>&1 | tail -10
```

Expected: all specs PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/image_server.go internal/mcp/mcp_test.go
git commit -m "feat(mcp): add ImageServer with list_images tool"
```

---

## Task 3: Wire MCP ImageServer in cmd/main.go

**Files:**
- Modify: `cmd/main.go`

- [ ] **Step 1: Instantiate and mount the image MCP server**

Find the block in `cmd/main.go` where `dnsMcpServer`, `alertsMcpServer` etc. are created (around line 763). Add the image server:

```go
// existing lines:
dnsMcpServer := mcp.NewDNSServer(fqdnStore, portalStore)
alertsMcpServer := mcp.NewAlertsServer(alertmanagerStore)
metricsMcpServer := mcp.NewMetricsServer(ctrlmetrics.Registry)
releasesMcpServer := mcp.NewReleasesServer(releaseStore)
netpolMcpServer := mcp.NewNetpolServer(flowGraphStore)
statusMcpServer := mcp.NewStatusServer(componentStore, maintenanceStore, incidentStore)
// ADD:
imageMcpServer := mcp.NewImageServer(imageStore)
```

- [ ] **Step 2: Mount the handler**

In the `switch mcpTransport` / `case "streamable-http":` block, after the existing `MountHandler` calls, add:

```go
webServer.MountHandler("/mcp/image", imageMcpServer.Handler())
```

Also update the log statement that lists MCP endpoints (look for the `setupLog.Info` call listing `"dns"`, `"alerts"` etc.) to include `"image", "/mcp/image"`.

- [ ] **Step 3: Build to confirm no compilation errors**

```bash
cd /Users/davidsabatie/Projects/go/src/github.com/golgoth31/sreportal
go build ./cmd/... 2>&1
```

Expected: exits with code 0, no output.

- [ ] **Step 4: Commit**

```bash
git add cmd/main.go
git commit -m "feat(mcp): mount image MCP server at /mcp/image"
```

---

## Task 4: Webhook validator — failing test

**Files:**
- Create: `internal/webhook/v1alpha1/imageinventory_webhook_test.go`

- [ ] **Step 1: Create the test file**

```go
/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

var _ = Describe("ImageInventory Webhook", func() {
	var (
		validator ImageInventoryCustomValidator
		obj       *sreportalv1alpha1.ImageInventory
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(sreportalv1alpha1.AddToScheme(scheme)).To(Succeed())
		portal := &sreportalv1alpha1.Portal{
			ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
			Spec:       sreportalv1alpha1.PortalSpec{Title: "Main Portal"},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()
		validator = ImageInventoryCustomValidator{client: fakeClient}

		obj = &sreportalv1alpha1.ImageInventory{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-inventory",
				Namespace: "default",
			},
			Spec: sreportalv1alpha1.ImageInventorySpec{
				PortalRef: "main",
			},
		}
	})

	Context("ValidateCreate", func() {
		It("should accept when portal exists", func() {
			warnings, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("should reject when portalRef is empty", func() {
			obj.Spec.PortalRef = ""
			_, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("portalRef"))
		})

		It("should reject when portal does not exist", func() {
			obj.Spec.PortalRef = "missing"
			_, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("ValidateUpdate", func() {
		It("should accept when new portalRef exists", func() {
			warnings, err := validator.ValidateUpdate(context.Background(), obj.DeepCopy(), obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("should reject when new portalRef does not exist", func() {
			newObj := obj.DeepCopy()
			newObj.Spec.PortalRef = "missing"
			_, err := validator.ValidateUpdate(context.Background(), obj, newObj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("ValidateDelete", func() {
		It("should always accept deletion", func() {
			warnings, err := validator.ValidateDelete(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})
	})
})
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
cd /Users/davidsabatie/Projects/go/src/github.com/golgoth31/sreportal
go test ./internal/webhook/... -run "ImageInventory" -v 2>&1 | head -20
```

Expected: compilation error — `ImageInventoryCustomValidator` undefined.

---

## Task 5: Webhook validator — implementation

**Files:**
- Create: `internal/webhook/v1alpha1/imageinventory_webhook.go`

- [ ] **Step 1: Create the file**

```go
/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/golgoth31/sreportal/internal/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// nolint:unused
var imageinventorylog = log.Default().WithName("imageinventory-resource")

// SetupImageInventoryWebhookWithManager registers the validation webhook for ImageInventory in the manager.
func SetupImageInventoryWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &sreportalv1alpha1.ImageInventory{}).
		WithValidator(&ImageInventoryCustomValidator{client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha1-imageinventory,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=imageinventories,verbs=create;update,versions=v1alpha1,name=vimageinventory-v1alpha1.kb.io,admissionReviewVersions=v1

// ImageInventoryCustomValidator validates the ImageInventory resource when it is created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ImageInventoryCustomValidator struct {
	client client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ImageInventory.
func (v *ImageInventoryCustomValidator) ValidateCreate(ctx context.Context, obj *sreportalv1alpha1.ImageInventory) (admission.Warnings, error) {
	imageinventorylog.Info("Validation for ImageInventory upon creation", "name", obj.GetName())

	return v.validatePortalRef(ctx, obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ImageInventory.
func (v *ImageInventoryCustomValidator) ValidateUpdate(ctx context.Context, _, newObj *sreportalv1alpha1.ImageInventory) (admission.Warnings, error) {
	imageinventorylog.Info("Validation for ImageInventory upon update", "name", newObj.GetName())

	return v.validatePortalRef(ctx, newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ImageInventory.
func (v *ImageInventoryCustomValidator) ValidateDelete(_ context.Context, obj *sreportalv1alpha1.ImageInventory) (admission.Warnings, error) {
	imageinventorylog.Info("Validation for ImageInventory upon deletion", "name", obj.GetName())

	return nil, nil
}

// validatePortalRef checks that the referenced portal exists.
func (v *ImageInventoryCustomValidator) validatePortalRef(ctx context.Context, obj *sreportalv1alpha1.ImageInventory) (admission.Warnings, error) {
	if obj.Spec.PortalRef == "" {
		return nil, fmt.Errorf("spec.portalRef is required")
	}

	var portal sreportalv1alpha1.Portal
	key := types.NamespacedName{
		Name:      obj.Spec.PortalRef,
		Namespace: obj.Namespace,
	}

	if err := v.client.Get(ctx, key, &portal); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("referenced portal %q not found in namespace %q", obj.Spec.PortalRef, obj.Namespace)
		}
		return nil, fmt.Errorf("failed to check portal reference: %w", err)
	}

	return nil, nil
}
```

- [ ] **Step 2: Run the tests to confirm they pass**

```bash
cd /Users/davidsabatie/Projects/go/src/github.com/golgoth31/sreportal
go test ./internal/webhook/... -run "ImageInventory" -v 2>&1 | tail -15
```

Expected: all ImageInventory Webhook specs PASS.

- [ ] **Step 3: Run full webhook suite for regressions**

```bash
go test ./internal/webhook/... -v 2>&1 | tail -10
```

Expected: all specs PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/webhook/v1alpha1/imageinventory_webhook.go internal/webhook/v1alpha1/imageinventory_webhook_test.go
git commit -m "feat(webhook): add ImageInventory portalRef validation webhook"
```

---

## Task 6: Wire webhook in suite + cmd/main.go

**Files:**
- Modify: `internal/webhook/v1alpha1/webhook_suite_test.go`
- Modify: `cmd/main.go`

- [ ] **Step 1: Register the webhook in the envtest suite**

In `internal/webhook/v1alpha1/webhook_suite_test.go`, find the block after `SetupDNSWebhookWithManager(mgr)` (around line 116) and add:

```go
err = SetupImageInventoryWebhookWithManager(mgr)
Expect(err).NotTo(HaveOccurred())
```

So the block becomes:
```go
err = SetupDNSWebhookWithManager(mgr)
Expect(err).NotTo(HaveOccurred())

err = SetupImageInventoryWebhookWithManager(mgr)
Expect(err).NotTo(HaveOccurred())

// +kubebuilder:scaffold:webhook
```

- [ ] **Step 2: Register the webhook in cmd/main.go**

Find the webhook registration block (around line 572):
```go
if err := webhookv1alpha1.SetupDNSWebhookWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create webhook", "webhook", "DNS")
    ...
}
if err := webhookv1alpha1.SetupPortalWebhookWithManager(mgr); err != nil {
    ...
}
if err := webhookv1alpha1.SetupReleaseWebhookWithManager(mgr, operatorConfig.Release.Types); err != nil {
    ...
}
```

Add after the Release webhook:
```go
if err := webhookv1alpha1.SetupImageInventoryWebhookWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create webhook", "webhook", "ImageInventory")
    os.Exit(1)
}
```

- [ ] **Step 3: Regenerate manifests (kubebuilder marker → webhook config)**

```bash
cd /Users/davidsabatie/Projects/go/src/github.com/golgoth31/sreportal
make helm
```

Expected: exits cleanly; `config/webhook/manifests.yaml` is updated with the new `vimageinventory-v1alpha1.kb.io` entry.

- [ ] **Step 4: Build to confirm no compilation errors**

```bash
go build ./cmd/... 2>&1
```

Expected: exits with code 0, no output.

- [ ] **Step 5: Commit**

```bash
git add cmd/main.go internal/webhook/v1alpha1/webhook_suite_test.go config/webhook/manifests.yaml helm/
git commit -m "feat(webhook): register ImageInventory webhook in manager and envtest suite"
```

---

## Task 7: Prometheus metrics — definition

**Files:**
- Modify: `internal/metrics/metrics.go`

- [ ] **Step 1: Add image metric variables**

In `internal/metrics/metrics.go`, after the `AlertsFetchErrorsTotal` block (around line 129) and before `PortalsTotal`, add:

```go
// --- Image inventory metrics ---

var (
	// ImageImagesTotal tracks the number of distinct images per portal and tag type.
	ImageImagesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "image",
			Name:      "images_total",
			Help:      "Number of distinct images per portal and tag type.",
		},
		[]string{"portal", "tag_type"},
	)

	// ImageInventoryScanTotal counts full scan attempts per ImageInventory resource and result.
	ImageInventoryScanTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "image",
			Name:      "inventory_scan_total",
			Help:      "Total number of full scan attempts per ImageInventory resource and result (success, error).",
		},
		[]string{"inventory", "result"},
	)
)
```

- [ ] **Step 2: Register the new metrics in the init() function**

In the `init()` function at the bottom of `internal/metrics/metrics.go`, add the new metrics to the `MustRegister` call after `AlertsFetchErrorsTotal`:

```go
// Image
ImageImagesTotal,
ImageInventoryScanTotal,
```

- [ ] **Step 3: Run tests to confirm no registration conflicts**

```bash
cd /Users/davidsabatie/Projects/go/src/github.com/golgoth31/sreportal
go test ./internal/metrics/... -v 2>&1
```

Expected: PASS (or no tests yet, exits 0).

- [ ] **Step 4: Commit**

```bash
git add internal/metrics/metrics.go
git commit -m "feat(metrics): add image inventory prometheus metrics"
```

---

## Task 8: Prometheus metrics — emit in scan handler

**Files:**
- Modify: `internal/controller/imageinventory/chain/scan_workloads.go`

- [ ] **Step 1: Add import and metric emission**

In `internal/controller/imageinventory/chain/scan_workloads.go`, add the metrics import:

```go
"github.com/golgoth31/sreportal/internal/metrics"
```

Then in the `Handle` method, replace:
```go
func (h *ScanWorkloadsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource
	byWorkload, err := h.scanAll(ctx, inv)
	if err != nil {
		wrapped := fmt.Errorf("full scan: %w", err)
		_ = statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionFalse, ReasonScanFailed, wrapped.Error())
		return wrapped
	}
	if err := h.store.ReplaceAll(ctx, inv.Spec.PortalRef, byWorkload); err != nil {
		wrapped := fmt.Errorf("replace store projection: %w", err)
		_ = statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionFalse, ReasonScanFailed, wrapped.Error())
		return wrapped
	}
	return nil
}
```

with:
```go
func (h *ScanWorkloadsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource
	byWorkload, err := h.scanAll(ctx, inv)
	if err != nil {
		metrics.ImageInventoryScanTotal.WithLabelValues(inv.Name, "error").Inc()
		wrapped := fmt.Errorf("full scan: %w", err)
		_ = statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionFalse, ReasonScanFailed, wrapped.Error())
		return wrapped
	}
	if err := h.store.ReplaceAll(ctx, inv.Spec.PortalRef, byWorkload); err != nil {
		metrics.ImageInventoryScanTotal.WithLabelValues(inv.Name, "error").Inc()
		wrapped := fmt.Errorf("replace store projection: %w", err)
		_ = statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionFalse, ReasonScanFailed, wrapped.Error())
		return wrapped
	}

	metrics.ImageInventoryScanTotal.WithLabelValues(inv.Name, "success").Inc()
	emitImageTotals(inv.Spec.PortalRef, byWorkload)
	return nil
}

// emitImageTotals updates the ImageImagesTotal gauge for the given portal.
// It resets all tag-type counters to zero first so stale values don't linger.
func emitImageTotals(portalRef string, byWorkload map[domainimage.WorkloadKey][]domainimage.ImageView) {
	tagCounts := map[domainimage.TagType]float64{}
	for _, views := range byWorkload {
		for _, v := range views {
			tagCounts[v.TagType]++
		}
	}
	for _, tt := range []domainimage.TagType{
		domainimage.TagTypeSemver,
		domainimage.TagTypeCommit,
		domainimage.TagTypeDigest,
		domainimage.TagTypeLatest,
	} {
		metrics.ImageImagesTotal.WithLabelValues(portalRef, string(tt)).Set(tagCounts[tt])
	}
}
```

- [ ] **Step 2: Run the existing scan_workloads tests to confirm no regressions**

```bash
cd /Users/davidsabatie/Projects/go/src/github.com/golgoth31/sreportal
go test ./internal/controller/imageinventory/... -v 2>&1 | tail -15
```

Expected: all tests PASS.

- [ ] **Step 3: Build to confirm no compilation errors**

```bash
go build ./cmd/... 2>&1
```

Expected: exits with code 0.

- [ ] **Step 4: Commit**

```bash
git add internal/controller/imageinventory/chain/scan_workloads.go
git commit -m "feat(metrics): emit image scan metrics from ScanWorkloadsHandler"
```

---

## Task 9: Colored tag-type badges in the UI

**Files:**
- Modify: `web/src/features/image/ui/ImageCard.tsx`

- [ ] **Step 1: Write a test for the badge color helper**

Open `web/src/features/image/domain/image.types.test.ts` and add:

```typescript
import { describe, it, expect } from "vitest";
import { tagTypeBadgeClass } from "../ui/ImageCard";

describe("tagTypeBadgeClass", () => {
  it("returns green classes for semver", () => {
    expect(tagTypeBadgeClass("semver")).toContain("green");
  });

  it("returns blue classes for commit", () => {
    expect(tagTypeBadgeClass("commit")).toContain("blue");
  });

  it("returns purple classes for digest", () => {
    expect(tagTypeBadgeClass("digest")).toContain("purple");
  });

  it("returns amber classes for latest", () => {
    expect(tagTypeBadgeClass("latest")).toContain("amber");
  });
});
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /Users/davidsabatie/Projects/go/src/github.com/golgoth31/sreportal
npm test --prefix web -- --reporter=verbose --run 2>&1 | grep -A5 "tagTypeBadgeClass"
```

Expected: fail — `tagTypeBadgeClass` is not exported from `ImageCard`.

- [ ] **Step 3: Update ImageCard.tsx**

Replace the entire content of `web/src/features/image/ui/ImageCard.tsx` with:

```tsx
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { TagType } from "../domain/image.types";
import type { Image } from "../domain/image.types";

interface ImageCardProps {
  image: Image;
}

export function tagTypeBadgeClass(tagType: TagType): string {
  const classes: Record<TagType, string> = {
    semver:
      "border-green-200 bg-green-100 text-green-800 dark:border-green-800 dark:bg-green-900/30 dark:text-green-300",
    commit:
      "border-blue-200 bg-blue-100 text-blue-800 dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-300",
    digest:
      "border-purple-200 bg-purple-100 text-purple-800 dark:border-purple-800 dark:bg-purple-900/30 dark:text-purple-300",
    latest:
      "border-amber-200 bg-amber-100 text-amber-800 dark:border-amber-800 dark:bg-amber-900/30 dark:text-amber-300",
  };
  return classes[tagType];
}

export function ImageCard({ image }: ImageCardProps) {
  const shortName = image.repository.split("/").at(-1) ?? image.repository;
  return (
    <div className="rounded-lg border bg-card p-4 flex flex-col gap-2 shadow-xs">
      <div className="flex items-center justify-between gap-2">
        <p className="font-medium text-sm">{shortName}</p>
        <Badge variant="outline" className={cn(tagTypeBadgeClass(image.tagType))}>
          {image.tagType}
        </Badge>
      </div>
      <p className="text-xs text-muted-foreground font-mono break-all">
        {image.repository}:{image.tag}
      </p>
      <p className="text-xs text-muted-foreground">
        {image.workloads.length} workload{image.workloads.length > 1 ? "s" : ""}
      </p>
    </div>
  );
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd /Users/davidsabatie/Projects/go/src/github.com/golgoth31/sreportal
npm test --prefix web -- --reporter=verbose --run 2>&1 | grep -A10 "tagTypeBadgeClass"
```

Expected: all 4 tagTypeBadgeClass tests PASS.

- [ ] **Step 5: Run full web test suite for regressions**

```bash
npm test --prefix web -- --run 2>&1 | tail -10
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/features/image/ui/ImageCard.tsx web/src/features/image/domain/image.types.test.ts
git commit -m "feat(ui): add colored tag-type badges in ImageCard"
```

---

## Self-Review

**Spec coverage:**

| Requirement | Task |
|---|---|
| MCP server at `/mcp/image` with `list_images` tool | Tasks 1–3 |
| `list_images` filters: portal, search, registry, tag_type | Task 1 (test), Task 2 (impl) |
| MCP session metrics (MCPSessionsActive label "image") | Task 2 (impl, hooks) |
| Webhook validates `spec.portalRef` exists on create/update | Tasks 4–5 |
| Webhook accepts any deletion | Task 4 (test) |
| Webhook registered in envtest suite | Task 6 |
| Webhook registered in cmd/main.go | Task 6 |
| Manifests regenerated | Task 6 |
| `ImageImagesTotal` gauge (portal, tag_type) | Task 7 |
| `ImageInventoryScanTotal` counter (inventory, result) | Task 7 |
| Metrics emitted on each scan | Task 8 |
| `emitImageTotals` resets all tag types to avoid stale values | Task 8 |
| Colored badges in ImageCard (semver=green, commit=blue, digest=purple, latest=amber) | Task 9 |
| Dark-mode support for colored badges | Task 9 |

**Placeholder scan:** None found.

**Type consistency:**
- `ImageInventoryCustomValidator.client` — defined in Task 5, used in test at Task 4 ✓
- `tagTypeBadgeClass` — exported from `ImageCard.tsx` in Task 9, imported in test at Task 9 ✓
- `domainimage.TagTypeSemver/Commit/Digest/Latest` — used in Task 1 test and Task 8, defined in existing `internal/domain/image/read_model.go` ✓
- `metrics.ImageImagesTotal` / `metrics.ImageInventoryScanTotal` — defined in Task 7, used in Task 8 ✓
