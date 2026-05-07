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

package chain_test

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/imageregistry/chain"
	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/registry"
)

var _ = Describe("ResolveLatestVersionsHandler", func() {
	var (
		ctx     context.Context
		limiter *registry.HostLimiter
	)

	BeforeEach(func() {
		ctx = context.Background()
		limiter = registry.NewHostLimiter()
	})

	makeIR := func(host string) *sreportalv1alpha1.ImageRegistry {
		return &sreportalv1alpha1.ImageRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: tTestName, Namespace: tNsDefault},
			Spec: sreportalv1alpha1.ImageRegistrySpec{
				Host:      host,
				PortalRef: tPortalMain,
				Namespace: tNsDefault,
			},
		}
	}

	Context("no due images", func() {
		It("is a no-op", func() {
			client := &fakeRegistryClient{}
			h := chain.NewResolveLatestVersionsHandler(client, limiter, nil, ctx)
			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{
				Resource: makeIR(tRegistryGhcr),
			}
			Expect(h.Handle(ctx, rc)).To(Succeed())
			Expect(rc.Data.Resolutions).To(BeNil())
		})
	})

	Context("semver image with newer version available", func() {
		It("resolves the latest version and sets UpgradeAvailable", func() {
			client := &fakeRegistryClient{
				tags: map[string][]string{
					"ghcr.io/myorg/myapp": {tVersion100, "1.1.0", "1.2.0", "latest"},
				},
			}
			h := chain.NewResolveLatestVersionsHandler(client, limiter, nil, ctx)

			dueImages := []chain.DueImage{
				{
					Spec: sreportalv1alpha1.ImageRegistrySpecEntry{
						Key:           "key1",
						OriginalImage: tImgGhcrMyorgMyapp,
						MutatedImage:  tImgGhcrMyorgMyapp,
						Repository:    "myorg/myapp",
						OriginalTag:   tVersion100,
						TagType:       tTagTypeSemver,
						ChangeType:    tChangeTypeNone,
					},
				},
			}
			resolutions := h.ResolveSync(ctx, tRegistryGhcr, dueImages)
			Expect(resolutions).To(HaveKey("key1"))
			res := resolutions["key1"]
			Expect(res.LatestVersion).To(Equal("1.2.0"))
			Expect(res.UpgradeAvailable).To(BeTrue())
			Expect(res.LastError).To(BeEmpty())
		})
	})

	Context("non-semver tag", func() {
		It("marks as checked with no version", func() {
			client := &fakeRegistryClient{}
			h := chain.NewResolveLatestVersionsHandler(client, limiter, nil, ctx)

			dueImages := []chain.DueImage{
				{
					Spec: sreportalv1alpha1.ImageRegistrySpecEntry{
						Key:          "key-digest",
						MutatedImage: "ghcr.io/myorg/myapp@sha256:abc",
						TagType:      "digest",
						ChangeType:   tChangeTypeNone,
					},
				},
			}
			resolutions := h.ResolveSync(ctx, tRegistryGhcr, dueImages)
			res := resolutions["key-digest"]
			Expect(res.LatestVersion).To(BeEmpty())
			Expect(res.UpgradeAvailable).To(BeFalse())
			Expect(res.LastError).To(BeEmpty())
			Expect(res.LastCheckedAt).NotTo(BeZero())
		})
	})

	Context("registry lookup error", func() {
		It("records the error in LastError", func() {
			lookupErr := errors.New("network failure")
			client := &fakeRegistryClient{err: lookupErr}
			h := chain.NewResolveLatestVersionsHandler(client, limiter, nil, ctx)

			dueImages := []chain.DueImage{
				{
					Spec: sreportalv1alpha1.ImageRegistrySpecEntry{
						Key:          "key-err",
						MutatedImage: tImgGhcrMyorgMyapp,
						Repository:   "myorg/myapp",
						OriginalTag:  tVersion100,
						TagType:      tTagTypeSemver,
						ChangeType:   tChangeTypeNone,
					},
				},
			}
			resolutions := h.ResolveSync(ctx, tRegistryGhcr, dueImages)
			res := resolutions["key-err"]
			Expect(res.LastError).NotTo(BeEmpty())
			Expect(res.LatestVersion).To(BeEmpty())
		})
	})

	Context("rate-limited registry", func() {
		It("records a LastError with rate-limited info", func() {
			rateLimitErr := fmt.Errorf("list tags: %w", domainimageregistry.ErrRateLimited)
			client := &fakeRegistryClient{err: rateLimitErr}
			h := chain.NewResolveLatestVersionsHandler(client, limiter, nil, ctx)

			dueImages := []chain.DueImage{
				{
					Spec: sreportalv1alpha1.ImageRegistrySpecEntry{
						Key:          "key-rate",
						MutatedImage: "docker.io/library/nginx:1.25",
						Repository:   "library/nginx",
						OriginalTag:  "1.25",
						TagType:      tTagTypeSemver,
						ChangeType:   tChangeTypeNone,
					},
				},
			}
			resolutions := h.ResolveSync(ctx, "docker.io", dueImages)
			res := resolutions["key-rate"]
			Expect(res.LastError).NotTo(BeEmpty())
		})
	})

	Context("injected image (OriginalImage empty)", func() {
		It("uses MutatedImage as lookup target", func() {
			client := &fakeRegistryClient{
				tags: map[string][]string{
					"ghcr.io/istio/proxy": {"1.19.0", "1.20.0"},
				},
			}
			h := chain.NewResolveLatestVersionsHandler(client, limiter, nil, ctx)

			dueImages := []chain.DueImage{
				{
					Spec: sreportalv1alpha1.ImageRegistrySpecEntry{
						Key:           "key-injected",
						OriginalImage: "", // injected: no original
						MutatedImage:  "ghcr.io/istio/proxy:1.19.0",
						Repository:    "istio/proxy",
						OriginalTag:   "1.19.0",
						TagType:       tTagTypeSemver,
						ChangeType:    "injected",
					},
				},
			}
			resolutions := h.ResolveSync(ctx, tRegistryGhcr, dueImages)
			res := resolutions["key-injected"]
			Expect(res.LatestVersion).To(Equal("1.20.0"))
			Expect(res.UpgradeAvailable).To(BeTrue())
		})
	})
})
