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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
	"github.com/golgoth31/sreportal/internal/tlsutil"
)

// FetchRemoteImagesHandler fetches images from a remote SRE Portal via the
// ImageService Connect API and populates ChainData.ByWorkload. It is a no-op
// when the inventory is local (Spec.IsRemote == false).
type FetchRemoteImagesHandler struct {
	k8sReader         client.Reader
	remoteClientCache *remoteclient.Cache
}

// NewFetchRemoteImagesHandler creates a new FetchRemoteImagesHandler.
func NewFetchRemoteImagesHandler(k8sReader client.Reader, remoteClientCache *remoteclient.Cache) *FetchRemoteImagesHandler {
	return &FetchRemoteImagesHandler{
		k8sReader:         k8sReader,
		remoteClientCache: remoteClientCache,
	}
}

// Handle implements reconciler.Handler.
func (h *FetchRemoteImagesHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource
	if !inv.Spec.IsRemote {
		return nil
	}

	logger := log.FromContext(ctx).WithName("fetch-remote-images")

	portal := &sreportalv1alpha1.Portal{}
	portalKey := types.NamespacedName{Name: inv.Spec.PortalRef, Namespace: inv.Namespace}
	if err := h.k8sReader.Get(ctx, portalKey, portal); err != nil {
		return fmt.Errorf("get portal %s: %w", portalKey, err)
	}
	if portal.Spec.Remote == nil {
		return fmt.Errorf("portal %s has no remote configuration but ImageInventory is marked as remote", portalKey.Name)
	}

	remoteClient, err := h.remoteClientFor(ctx, portal)
	if err != nil {
		return fmt.Errorf("build remote client for portal %s: %w", portalKey.Name, err)
	}

	baseURL := portal.Spec.Remote.URL
	portalName := portal.Spec.Remote.Portal
	logger.V(1).Info("fetching images from remote portal", "url", baseURL, "portalName", portalName)

	result, err := remoteClient.FetchImages(ctx, baseURL, portalName)
	if err != nil {
		return fmt.Errorf("fetch remote images from %s: %w", baseURL, err)
	}

	rc.Data.ByWorkload = remoteImagesToByWorkload(inv.Spec.PortalRef, result.Images)
	logger.V(1).Info("fetched remote images", "workloads", len(rc.Data.ByWorkload))
	return nil
}

// remoteImagesToByWorkload converts remote ImageService responses into the
// per-workload domain projection consumed by ProjectImagesHandler.
func remoteImagesToByWorkload(portalRef string, images []*remoteclient.RemoteImage) map[domainimage.WorkloadKey][]domainimage.ImageView {
	out := make(map[domainimage.WorkloadKey][]domainimage.ImageView)
	for _, img := range images {
		if img == nil {
			continue
		}
		view := domainimage.ImageView{
			PortalRef:  portalRef,
			Registry:   img.Registry,
			Repository: img.Repository,
			Tag:        img.Tag,
			TagType:    domainimage.TagType(img.TagType),
		}
		for _, w := range img.Workloads {
			ref := domainimage.WorkloadRef{
				Kind:      w.Kind,
				Namespace: w.Namespace,
				Name:      w.Name,
				Container: w.Container,
				Source:    domainimage.ContainerSource(w.Source),
			}
			view.Workloads = append(view.Workloads, ref)
			wk := domainimage.WorkloadKey{Kind: w.Kind, Namespace: w.Namespace, Name: w.Name}
			out[wk] = append(out[wk], view)
		}
	}
	return out
}

// remoteClientFor returns a cached remoteclient configured with TLS from the Portal spec.
func (h *FetchRemoteImagesHandler) remoteClientFor(ctx context.Context, portal *sreportalv1alpha1.Portal) (*remoteclient.Client, error) {
	if portal.Spec.Remote.TLS == nil {
		return h.remoteClientCache.Fallback(), nil
	}

	key := portal.Namespace + "/" + portal.Name
	versions, err := tlsutil.SecretVersions(ctx, h.k8sReader, portal.Namespace, portal.Spec.Remote.TLS)
	if err != nil {
		return nil, fmt.Errorf("read TLS secret versions: %w", err)
	}

	if cached := h.remoteClientCache.Get(key, versions); cached != nil {
		return cached, nil
	}

	tlsConfig, err := tlsutil.BuildTLSConfig(ctx, h.k8sReader, portal.Namespace, portal.Spec.Remote.TLS)
	if err != nil {
		return nil, fmt.Errorf("build TLS config: %w", err)
	}

	c := remoteclient.NewClient(remoteclient.WithTLSConfig(tlsConfig))
	h.remoteClientCache.Put(key, versions, c)

	return c, nil
}
