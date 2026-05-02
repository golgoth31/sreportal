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

package chain

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ProjectImagesHandler writes ChainData.ByWorkload to the readstore and emits
// metrics. It is the single write path for both local and remote sources.
type ProjectImagesHandler struct {
	client client.Client
	store  domainimage.ImageWriter
}

// NewProjectImagesHandler constructs a ProjectImagesHandler.
func NewProjectImagesHandler(c client.Client, store domainimage.ImageWriter) *ProjectImagesHandler {
	return &ProjectImagesHandler{client: c, store: store}
}

// Handle implements reconciler.Handler.
func (h *ProjectImagesHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource
	byWorkload := rc.Data.ByWorkload
	if byWorkload == nil {
		byWorkload = map[domainimage.WorkloadKey][]domainimage.ImageView{}
	}

	if err := h.store.ReplaceAll(ctx, inv.Spec.PortalRef, byWorkload); err != nil {
		metrics.ImageInventorySyncTotal.WithLabelValues(inv.Name, "error").Inc()
		wrapped := fmt.Errorf("replace store projection: %w", err)
		_ = statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionFalse, ReasonProjectionFailed, wrapped.Error())
		return wrapped
	}

	metrics.ImageInventorySyncTotal.WithLabelValues(inv.Name, "success").Inc()
	emitImageTotals(inv.Spec.PortalRef, byWorkload)
	return nil
}

// emitImageTotals updates the ImageImagesTotal gauge for the given portal.
// It resets all known tag-type counters first so stale values don't linger.
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
		domainimage.TagTypeOther,
	} {
		metrics.ImageImagesTotal.WithLabelValues(portalRef, string(tt)).Set(tagCounts[tt])
	}
}
