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
	"k8s.io/apimachinery/pkg/types"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/imageregistry/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

var _ = Describe("UpdateStatusHandler", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("never mutates Status.Images (ownership belongs to ResolveLatestVersionsHandler.patchStatus)", func() {
		// Pre-populated status mimicking a successful resolve cycle.
		// Use second precision because metav1.Time serializes at RFC3339
		// second precision through the fake client.
		checkedAt := metav1.NewTime(time.Now().Add(-time.Hour).Truncate(time.Second))
		preExisting := []sreportalv1alpha1.ImageRegistryStatusEntry{
			{
				Key:              "k1",
				LatestVersion:    "1.2.3",
				UpgradeAvailable: true,
				LastCheckedAt:    &checkedAt,
			},
			{
				Key:           "k2",
				LatestVersion: "2.0.0",
				LastCheckedAt: &checkedAt,
			},
		}
		ir := &sreportalv1alpha1.ImageRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: tTestName, Namespace: tNsDefault},
			Spec: sreportalv1alpha1.ImageRegistrySpec{
				Host:      tRegistryGhcr,
				PortalRef: tPortalMain,
				Namespace: tNsDefault,
				Images: []sreportalv1alpha1.ImageRegistrySpecEntry{
					// k1 already in status.
					{Key: "k1", MutatedImage: tImgGhcrMyorgMyapp, TagType: tTagTypeSemver, ChangeType: tChangeTypeNone},
					// k2 already in status.
					{Key: "k2", MutatedImage: tImgGhcrMyorgMyapp, TagType: tTagTypeSemver, ChangeType: tChangeTypeMutated, OriginalImage: tImgGhcrMyorgMyapp},
					// k3 NOT in status — would have been a placeholder candidate before the fix.
					{Key: "k3", MutatedImage: tImgGhcrMyorgMyapp, TagType: tTagTypeSemver, ChangeType: tChangeTypeInjected},
				},
			},
			Status: sreportalv1alpha1.ImageRegistryStatus{
				Images: preExisting,
			},
		}

		client := newFakeClient(ir)
		handler := chain.NewUpdateStatusHandler(client)

		rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: ir}
		Expect(handler.Handle(ctx, rc)).To(Succeed())

		var got sreportalv1alpha1.ImageRegistry
		Expect(client.Get(ctx, types.NamespacedName{Namespace: tNsDefault, Name: tTestName}, &got)).To(Succeed())

		// Status.Images must be byte-identical to the pre-existing slice — no
		// placeholder for k3 added, no entry dropped, no reorder. This is the
		// invariant: only ResolveLatestVersionsHandler.patchStatus owns Status.Images.
		Expect(got.Status.Images).To(Equal(preExisting))
	})

	It("computes counters from current Status.Images (not Spec.Images)", func() {
		checkedAt := metav1.NewTime(time.Now().Add(-time.Hour))
		ir := &sreportalv1alpha1.ImageRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: tTestName, Namespace: tNsDefault},
			Spec: sreportalv1alpha1.ImageRegistrySpec{
				Host:      tRegistryGhcr,
				PortalRef: tPortalMain,
				Namespace: tNsDefault,
				Images: []sreportalv1alpha1.ImageRegistrySpecEntry{
					{Key: "k1", MutatedImage: tImgGhcrMyorgMyapp, TagType: tTagTypeSemver, ChangeType: tChangeTypeNone},
					{Key: "k2", MutatedImage: tImgGhcrMyorgMyapp, TagType: tTagTypeSemver, ChangeType: tChangeTypeMutated, OriginalImage: tImgGhcrMyorgMyapp},
					{Key: "k3", MutatedImage: tImgGhcrMyorgMyapp, TagType: tTagTypeSemver, ChangeType: tChangeTypeInjected},
					// k4 in spec but never resolved yet.
					{Key: "k4", MutatedImage: tImgGhcrMyorgMyapp, TagType: tTagTypeSemver, ChangeType: tChangeTypeNone},
				},
			},
			Status: sreportalv1alpha1.ImageRegistryStatus{
				Images: []sreportalv1alpha1.ImageRegistryStatusEntry{
					{Key: "k1", UpgradeAvailable: true, LastCheckedAt: &checkedAt},
					{Key: "k2", UpgradeAvailable: false, LastCheckedAt: &checkedAt},
					{Key: "k3", UpgradeAvailable: true, LastCheckedAt: &checkedAt},
				},
			},
		}

		client := newFakeClient(ir)
		handler := chain.NewUpdateStatusHandler(client)

		rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: ir}
		Expect(handler.Handle(ctx, rc)).To(Succeed())

		var got sreportalv1alpha1.ImageRegistry
		Expect(client.Get(ctx, types.NamespacedName{Namespace: tNsDefault, Name: tTestName}, &got)).To(Succeed())

		// 3 entries in Status (k1, k2, k3); k4 not yet materialised by patchStatus.
		Expect(got.Status.ImageCount).To(Equal(int32(3)))
		Expect(got.Status.UpgradeAvailableCount).To(Equal(int32(2)))
		Expect(got.Status.MutatedCount).To(Equal(int32(1)))
		Expect(got.Status.InjectedCount).To(Equal(int32(1)))
	})

	It("is idempotent — second reconcile does not bump resourceVersion", func() {
		ir := &sreportalv1alpha1.ImageRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: tTestName, Namespace: tNsDefault},
			Spec: sreportalv1alpha1.ImageRegistrySpec{
				Host:      tRegistryGhcr,
				PortalRef: tPortalMain,
				Namespace: tNsDefault,
				Images:    []sreportalv1alpha1.ImageRegistrySpecEntry{},
			},
		}

		c := newFakeClient(ir)
		handler := chain.NewUpdateStatusHandler(c)

		// First reconcile: writes counters + Ready condition.
		rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: ir.DeepCopy()}
		Expect(handler.Handle(ctx, rc)).To(Succeed())

		var afterFirst sreportalv1alpha1.ImageRegistry
		Expect(c.Get(ctx, types.NamespacedName{Namespace: tNsDefault, Name: tTestName}, &afterFirst)).To(Succeed())

		// Second reconcile from the latest server state: nothing should change.
		rc2 := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: afterFirst.DeepCopy()}
		Expect(handler.Handle(ctx, rc2)).To(Succeed())

		var afterSecond sreportalv1alpha1.ImageRegistry
		Expect(c.Get(ctx, types.NamespacedName{Namespace: tNsDefault, Name: tTestName}, &afterSecond)).To(Succeed())

		Expect(afterSecond.ResourceVersion).To(Equal(afterFirst.ResourceVersion),
			"no-op reconcile must skip the status patch and not bump resourceVersion")
	})

	It("sets the Ready condition", func() {
		ir := &sreportalv1alpha1.ImageRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: tTestName, Namespace: tNsDefault},
			Spec: sreportalv1alpha1.ImageRegistrySpec{
				Host:      tRegistryGhcr,
				PortalRef: tPortalMain,
				Namespace: tNsDefault,
				Images:    []sreportalv1alpha1.ImageRegistrySpecEntry{},
			},
		}

		client := newFakeClient(ir)
		handler := chain.NewUpdateStatusHandler(client)

		rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, chain.ChainData]{Resource: ir}
		Expect(handler.Handle(ctx, rc)).To(Succeed())

		var got sreportalv1alpha1.ImageRegistry
		Expect(client.Get(ctx, types.NamespacedName{Namespace: tNsDefault, Name: tTestName}, &got)).To(Succeed())

		Expect(got.Status.Conditions).To(HaveLen(1))
		Expect(got.Status.Conditions[0].Type).To(Equal(chain.ConditionTypeReady))
		Expect(got.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
		Expect(got.Status.Conditions[0].Reason).To(Equal(chain.ReasonReconciled))
	})
})
