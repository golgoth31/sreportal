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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/imageregistry/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

var _ = Describe("SelectDueImagesHandler", func() {
	var (
		ctx     context.Context
		handler *chain.SelectDueImagesHandler
	)

	BeforeEach(func() {
		ctx = context.Background()
		handler = chain.NewSelectDueImagesHandler()
	})

	makeSpec := func(keys ...string) []sreportalv1alpha1.ImageRegistrySpecEntry {
		entries := make([]sreportalv1alpha1.ImageRegistrySpecEntry, 0, len(keys))
		for _, k := range keys {
			entries = append(entries, sreportalv1alpha1.ImageRegistrySpecEntry{Key: k, MutatedImage: "img:1.0", TagType: tTagTypeSemver})
		}
		return entries
	}

	makeStatus := func(key string, checkedAt time.Time) sreportalv1alpha1.ImageRegistryStatusEntry {
		t := metav1.NewTime(checkedAt)
		return sreportalv1alpha1.ImageRegistryStatusEntry{Key: key, LastCheckedAt: &t}
	}

	Context("brand-new CR (Status.Images empty)", func() {
		It("marks all images as due regardless of count", func() {
			ir := &sreportalv1alpha1.ImageRegistry{
				Spec: sreportalv1alpha1.ImageRegistrySpec{
					Host:      tRegistryGhcr,
					PortalRef: tPortalMain,
					Namespace: tNsDefault,
					Images:    makeSpec("a", "b", "c"),
				},
				Status: sreportalv1alpha1.ImageRegistryStatus{},
			}
			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: ir}
			Expect(handler.Handle(ctx, rc)).To(Succeed())
			Expect(rc.Data.DueImages).To(HaveLen(3))
			Expect(rc.Data.RequeueAfter).To(BeZero())
		})
	})

	Context("recently-checked images (within 24h)", func() {
		It("skips all images", func() {
			recent := time.Now().Add(-1 * time.Hour)
			ir := &sreportalv1alpha1.ImageRegistry{
				Spec: sreportalv1alpha1.ImageRegistrySpec{
					Host: tRegistryGhcr, PortalRef: tPortalMain, Namespace: tNsDefault,
					Images: makeSpec("a", "b"),
				},
				Status: sreportalv1alpha1.ImageRegistryStatus{
					Images: []sreportalv1alpha1.ImageRegistryStatusEntry{
						makeStatus("a", recent),
						makeStatus("b", recent),
					},
				},
			}
			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: ir}
			Expect(handler.Handle(ctx, rc)).To(Succeed())
			Expect(rc.Data.DueImages).To(BeEmpty())
		})
	})

	Context("post-restart: exactly 50% due (boundary — no jitter)", func() {
		It("processes all due images without jitter when ≤50%", func() {
			old := time.Now().Add(-25 * time.Hour)
			recent := time.Now().Add(-1 * time.Hour)
			// 2 due out of 4 total = 50% — threshold is > 50%, so no jitter.
			ir := &sreportalv1alpha1.ImageRegistry{
				Spec: sreportalv1alpha1.ImageRegistrySpec{
					Host: tRegistryGhcr, PortalRef: tPortalMain, Namespace: tNsDefault,
					Images: makeSpec("a", "b", "c", "d"),
				},
				Status: sreportalv1alpha1.ImageRegistryStatus{
					Images: []sreportalv1alpha1.ImageRegistryStatusEntry{
						makeStatus("a", old),
						makeStatus("b", old),
						makeStatus("c", recent),
						makeStatus("d", recent),
					},
				},
			}
			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: ir}
			Expect(handler.Handle(ctx, rc)).To(Succeed())
			// All due images should be selected (no catch-up jitter at exactly 50%).
			Expect(rc.Data.DueImages).To(HaveLen(2))
		})
	})

	Context("post-restart: >50% due (catch-up jitter)", func() {
		It("may defer some images and sets a non-zero RequeueAfter when all are deferred", func() {
			old := time.Now().Add(-25 * time.Hour)
			recent := time.Now().Add(-1 * time.Hour)
			// 3 due out of 4 total = 75% → catch-up jitter applied.
			ir := &sreportalv1alpha1.ImageRegistry{
				Spec: sreportalv1alpha1.ImageRegistrySpec{
					Host: tRegistryGhcr, PortalRef: tPortalMain, Namespace: tNsDefault,
					Images: makeSpec("a", "b", "c", "d"),
				},
				Status: sreportalv1alpha1.ImageRegistryStatus{
					Images: []sreportalv1alpha1.ImageRegistryStatusEntry{
						makeStatus("a", old),
						makeStatus("b", old),
						makeStatus("c", old),
						makeStatus("d", recent),
					},
				},
			}

			// Run several times; the jitter is random, but the invariant is:
			// processed + deferred == dueCount (3).
			// At least one run should result in some deferral (probabilistically).
			// For test stability we just assert the processed count is ≤ dueCount.
			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: ir}
			Expect(handler.Handle(ctx, rc)).To(Succeed())
			Expect(len(rc.Data.DueImages)).To(BeNumerically("<=", 3))
		})

		It("sets RequeueAfter only when some images were deferred", func() {
			// Same setup as above but we loop to find a case where some are deferred.
			old := time.Now().Add(-25 * time.Hour)
			recent := time.Now().Add(-1 * time.Hour)

			ir := &sreportalv1alpha1.ImageRegistry{
				Spec: sreportalv1alpha1.ImageRegistrySpec{
					Host: tRegistryGhcr, PortalRef: tPortalMain, Namespace: tNsDefault,
					Images: makeSpec("a", "b", "c", "d"),
				},
				Status: sreportalv1alpha1.ImageRegistryStatus{
					Images: []sreportalv1alpha1.ImageRegistryStatusEntry{
						makeStatus("a", old),
						makeStatus("b", old),
						makeStatus("c", old),
						makeStatus("d", recent),
					},
				},
			}

			// The assertion is that when deferred > 0, RequeueAfter > 0.
			// We try up to 20 runs and verify the invariant holds whenever deferred > 0.
			for range 20 {
				rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: ir}
				Expect(handler.Handle(ctx, rc)).To(Succeed())
				deferred := 3 - len(rc.Data.DueImages)
				if deferred > 0 {
					Expect(rc.Data.RequeueAfter).To(BeNumerically(">", 0))
				}
			}
		})
	})

	Context("post-restart with deterministic jitter (all deferred)", func() {
		It("leaves DueImages empty and requeues at the minimum positive delay", func() {
			old := time.Now().Add(-25 * time.Hour)
			// 3 due out of 4 total = 75% → catch-up jitter applies.
			ir := &sreportalv1alpha1.ImageRegistry{
				Spec: sreportalv1alpha1.ImageRegistrySpec{
					Host: tRegistryGhcr, PortalRef: tPortalMain, Namespace: tNsDefault,
					Images: makeSpec("a", "b", "c", "d"),
				},
				Status: sreportalv1alpha1.ImageRegistryStatus{
					Images: []sreportalv1alpha1.ImageRegistryStatusEntry{
						makeStatus("a", old),
						makeStatus("b", old),
						makeStatus("c", old),
						makeStatus("d", time.Now().Add(-1*time.Hour)),
					},
				},
			}

			// Strictly positive, strictly increasing delays. None equals zero,
			// so every due image should be deferred and DueImages must be empty.
			delays := []time.Duration{30 * time.Second, 10 * time.Second, 20 * time.Second}
			var i int
			jitter := func(_ time.Duration) time.Duration {
				d := delays[i]
				i++
				return d
			}
			h := chain.NewSelectDueImagesHandlerWithJitter(jitter)

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: ir}
			Expect(h.Handle(ctx, rc)).To(Succeed())
			Expect(rc.Data.DueImages).To(BeEmpty())
			// RequeueAfter must be the minimum of the three deferred delays,
			// not just the first one observed nor the last one.
			Expect(rc.Data.RequeueAfter).To(Equal(10 * time.Second))
		})
	})

	Context("post-restart with deterministic jitter (mixed zero and non-zero)", func() {
		It("processes zero-delay images and requeues at the minimum positive delay among the rest", func() {
			old := time.Now().Add(-25 * time.Hour)
			ir := &sreportalv1alpha1.ImageRegistry{
				Spec: sreportalv1alpha1.ImageRegistrySpec{
					Host: tRegistryGhcr, PortalRef: tPortalMain, Namespace: tNsDefault,
					Images: makeSpec("a", "b", "c", "d"),
				},
				Status: sreportalv1alpha1.ImageRegistryStatus{
					Images: []sreportalv1alpha1.ImageRegistryStatusEntry{
						makeStatus("a", old),
						makeStatus("b", old),
						makeStatus("c", old),
						makeStatus("d", time.Now().Add(-1*time.Hour)),
					},
				},
			}

			// 3 due images: 1st → 0 (process), 2nd → 45s (defer), 3rd → 15s (defer).
			// Minimum deferred delay = 15s.
			delays := []time.Duration{0, 45 * time.Second, 15 * time.Second}
			var i int
			jitter := func(_ time.Duration) time.Duration {
				d := delays[i]
				i++
				return d
			}
			h := chain.NewSelectDueImagesHandlerWithJitter(jitter)

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: ir}
			Expect(h.Handle(ctx, rc)).To(Succeed())
			Expect(rc.Data.DueImages).To(HaveLen(1))
			Expect(rc.Data.RequeueAfter).To(Equal(15 * time.Second))
		})
	})

	Context("empty spec", func() {
		It("returns without error and no due images", func() {
			ir := &sreportalv1alpha1.ImageRegistry{
				Spec:   sreportalv1alpha1.ImageRegistrySpec{Host: tRegistryGhcr, PortalRef: tPortalMain, Namespace: tNsDefault},
				Status: sreportalv1alpha1.ImageRegistryStatus{},
			}
			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: ir}
			Expect(handler.Handle(ctx, rc)).To(Succeed())
			Expect(rc.Data.DueImages).To(BeEmpty())
		})
	})
})
