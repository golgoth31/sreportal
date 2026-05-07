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
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
	"github.com/golgoth31/sreportal/internal/tlsutil"
)

// FetchRemoteImagesHandler fetches images from a remote SRE Portal via the
// ImageService Connect API and writes them directly to the readstore grouped
// by (host, namespace). It is a no-op when the inventory is local
// (Spec.IsRemote == false).
//
// Remote inventories DO NOT create child ImageRegistry CRs — those belong to
// the source portal's ImageInventory. We persist the (host, namespace) scopes
// in `inv.Status.Registries[]` (with empty Hash) so the finalizer can call
// RemoveForNamespace on each scope at deletion time.
type FetchRemoteImagesHandler struct {
	k8sClient         client.Client
	remoteClientCache *remoteclient.Cache
	store             domainimage.ImageWriter
}

// NewFetchRemoteImagesHandler creates a new FetchRemoteImagesHandler.
func NewFetchRemoteImagesHandler(
	c client.Client,
	remoteClientCache *remoteclient.Cache,
	store domainimage.ImageWriter,
) *FetchRemoteImagesHandler {
	return &FetchRemoteImagesHandler{
		k8sClient:         c,
		remoteClientCache: remoteClientCache,
		store:             store,
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
	if err := h.k8sClient.Get(ctx, portalKey, portal); err != nil {
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
		// Surface fetch errors on the Ready condition so users see why no
		// images appear in the dashboard.
		_ = statusutil.SetConditionAndPatch(ctx, h.k8sClient, inv, ReadyConditionType, metav1.ConditionFalse, ReasonScanFailed, err.Error())
		metrics.ImageInventorySyncTotal.WithLabelValues(inv.Name, "error").Inc()
		return fmt.Errorf("fetch remote images from %s: %w", baseURL, err)
	}

	scopes := groupRemoteImagesByHostNamespace(inv.Spec.PortalRef, result.Images)

	// Drop scopes that were present in the previous reconcile but are absent
	// from the new fetch (e.g. namespace deleted upstream).
	if err := h.dropMissingScopes(ctx, inv, scopes); err != nil {
		metrics.ImageInventorySyncTotal.WithLabelValues(inv.Name, "error").Inc()
		return err
	}

	// Replace each scope with its current view.
	keys := make([]scopeRef, 0, len(scopes))
	for k := range scopes {
		keys = append(keys, k)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if keys[i].Host != keys[j].Host {
			return keys[i].Host < keys[j].Host
		}
		return keys[i].Namespace < keys[j].Namespace
	})

	refs := make([]sreportalv1alpha1.ImageRegistryRef, 0, len(keys))
	for _, k := range keys {
		if err := h.store.ReplaceForNamespace(ctx, inv.Spec.PortalRef, k.Host, k.Namespace, scopes[k]); err != nil {
			metrics.ImageInventorySyncTotal.WithLabelValues(inv.Name, "error").Inc()
			return fmt.Errorf("readstore replace (host=%s ns=%s): %w", k.Host, k.Namespace, err)
		}
		refs = append(refs, sreportalv1alpha1.ImageRegistryRef{Host: k.Host, Namespace: k.Namespace})
	}

	// Persist Status.Registries so the next reconcile's dropMissingScopes can
	// detect orphan scopes (otherwise the readstore retains entries for scopes
	// that disappeared upstream).
	base := inv.DeepCopy()
	inv.Status.Registries = refs
	if err := h.k8sClient.Status().Patch(ctx, inv, client.MergeFrom(base)); err != nil {
		metrics.ImageInventorySyncTotal.WithLabelValues(inv.Name, "error").Inc()
		return fmt.Errorf("patch Status.Registries: %w", err)
	}

	metrics.ImageInventorySyncTotal.WithLabelValues(inv.Name, "success").Inc()
	logger.V(1).Info("fetched remote images", "scopes", len(scopes))
	return nil
}

// scopeRef is the in-memory grouping key used while building the per-scope
// ImageView slices. It mirrors readstore's (host, namespace) tuple.
type scopeRef struct {
	Host      string
	Namespace string
}

// groupRemoteImagesByHostNamespace flattens RemoteImage[] (each with its own
// workload list) into the (host, namespace) view groups expected by
// readstore.ReplaceForNamespace.
//
// A single RemoteImage may produce contributions in multiple namespaces when
// its workloads span them. Each (image, namespace) tuple becomes one
// ImageView with the workloads pruned to that namespace.
func groupRemoteImagesByHostNamespace(portalRef string, images []*remoteclient.RemoteImage) map[scopeRef][]domainimage.ImageView {
	out := make(map[scopeRef][]domainimage.ImageView)
	for _, img := range images {
		if img == nil {
			continue
		}
		// Bucket workloads by namespace so each ImageView lands in the
		// correct (host, namespace) scope.
		byNS := make(map[string][]domainimage.WorkloadRef)
		for _, w := range img.Workloads {
			byNS[w.Namespace] = append(byNS[w.Namespace], domainimage.WorkloadRef{
				Kind:      w.Kind,
				Namespace: w.Namespace,
				Name:      w.Name,
				Container: w.Container,
				Source:    domainimage.ContainerSource(w.Source),
			})
		}
		for ns, refs := range byNS {
			view := domainimage.ImageView{
				PortalRef:        portalRef,
				Registry:         img.Registry,
				Repository:       img.Repository,
				Tag:              img.Tag,
				TagType:          domainimage.TagType(img.TagType),
				Workloads:        refs,
				OriginalImage:    img.OriginalImage,
				ChangeType:       img.ChangeType,
				LatestVersion:    img.LatestVersion,
				LatestCheckedAt:  img.LatestCheckedAt,
				LatestError:      img.LatestError,
				UpgradeAvailable: img.UpgradeAvailable,
			}
			key := scopeRef{Host: img.Registry, Namespace: ns}
			out[key] = append(out[key], view)
		}
	}
	return out
}

// dropMissingScopes removes (from the readstore) any scope that was present
// in the previous reconcile (recorded in Status.Registries) but is absent
// from the latest scopes set.
func (h *FetchRemoteImagesHandler) dropMissingScopes(
	ctx context.Context,
	inv *sreportalv1alpha1.ImageInventory,
	scopes map[scopeRef][]domainimage.ImageView,
) error {
	keep := make(map[scopeRef]struct{}, len(scopes))
	for k := range scopes {
		keep[k] = struct{}{}
	}
	for _, prev := range inv.Status.Registries {
		k := scopeRef{Host: prev.Host, Namespace: prev.Namespace}
		if _, ok := keep[k]; ok {
			continue
		}
		if err := h.store.RemoveForNamespace(ctx, inv.Spec.PortalRef, k.Host, k.Namespace); err != nil {
			return fmt.Errorf("readstore remove orphan (host=%s ns=%s): %w", k.Host, k.Namespace, err)
		}
	}
	return nil
}

// remoteClientFor returns a cached remoteclient configured with TLS from the Portal spec.
func (h *FetchRemoteImagesHandler) remoteClientFor(ctx context.Context, portal *sreportalv1alpha1.Portal) (*remoteclient.Client, error) {
	if portal.Spec.Remote.TLS == nil {
		return h.remoteClientCache.Fallback(), nil
	}

	key := portal.Namespace + "/" + portal.Name
	versions, err := tlsutil.SecretVersions(ctx, h.k8sClient, portal.Namespace, portal.Spec.Remote.TLS)
	if err != nil {
		return nil, fmt.Errorf("read TLS secret versions: %w", err)
	}

	if cached := h.remoteClientCache.Get(key, versions); cached != nil {
		return cached, nil
	}

	tlsConfig, err := tlsutil.BuildTLSConfig(ctx, h.k8sClient, portal.Namespace, portal.Spec.Remote.TLS)
	if err != nil {
		return nil, fmt.Errorf("build TLS config: %w", err)
	}

	c := remoteclient.NewClient(remoteclient.WithTLSConfig(tlsConfig))
	h.remoteClientCache.Put(key, versions, c)

	return c, nil
}
