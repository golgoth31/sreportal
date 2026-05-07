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
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	imagestore "github.com/golgoth31/sreportal/internal/readstore/image"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

var _ = Describe("UpdateReadstoreHandler", func() {
	var (
		ctx   context.Context
		store *imagestore.Store
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = imagestore.NewStore()
	})

	makeIR := func() *sreportalv1alpha1.ImageRegistry {
		return &sreportalv1alpha1.ImageRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: tTestName, Namespace: tNsDefault},
			Spec: sreportalv1alpha1.ImageRegistrySpec{
				Host:      tRegistryGhcr,
				PortalRef: tPortalMain,
				Namespace: tNsDefault,
				Images: []sreportalv1alpha1.ImageRegistrySpecEntry{
					{
						Key:           "k1",
						OriginalImage: "ghcr.io/org/app:1.0.0",
						MutatedImage:  "ghcr.io/org/app:1.0.0",
						Repository:    "org/app",
						OriginalTag:   tVersion100,
						TagType:       tTagTypeSemver,
						ChangeType:    tChangeTypeNone,
					},
				},
			},
		}
	}

	It("writes resolved image views to the readstore", func() {
		h := chain.NewUpdateReadstoreHandler(store)
		ir := makeIR()

		resolvedAt := time.Now()
		rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{
			Resource: ir,
			Data: chain.ChainData{
				Resolutions: map[string]chain.Resolution{
					"k1": {
						Key:              "k1",
						LatestVersion:    "1.2.0",
						UpgradeAvailable: true,
						LastCheckedAt:    resolvedAt,
					},
				},
			},
		}
		Expect(h.Handle(ctx, rc)).To(Succeed())

		views, err := store.List(ctx, domainimage.ImageFilters{Portal: tPortalMain})
		Expect(err).NotTo(HaveOccurred())
		Expect(views).To(HaveLen(1))
		Expect(views[0].LatestVersion).To(Equal("1.2.0"))
		Expect(views[0].UpgradeAvailable).To(BeTrue())
		Expect(views[0].LatestCheckedAt).NotTo(BeNil())
	})

	It("preserves previous Status for non-due images", func() {
		h := chain.NewUpdateReadstoreHandler(store)
		ir := makeIR()

		prevChecked := metav1.NewTime(time.Now().Add(-2 * time.Hour))
		ir.Status.Images = []sreportalv1alpha1.ImageRegistryStatusEntry{
			{
				Key:           "k1",
				LatestVersion: "1.1.0",
				LastCheckedAt: &prevChecked,
			},
		}

		// No resolutions = k1 is non-due, should use status.
		rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{
			Resource: ir,
			Data:     chain.ChainData{Resolutions: map[string]chain.Resolution{}},
		}
		Expect(h.Handle(ctx, rc)).To(Succeed())

		views, err := store.List(ctx, domainimage.ImageFilters{Portal: tPortalMain})
		Expect(err).NotTo(HaveOccurred())
		Expect(views).To(HaveLen(1))
		Expect(views[0].LatestVersion).To(Equal("1.1.0"))
	})
})
