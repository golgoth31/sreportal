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
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
	"github.com/golgoth31/sreportal/internal/tlsutil"
)

// FetchRemoteGraphHandler fetches network flow data from a remote SRE Portal
// via the NetworkPolicyService Connect API and populates ChainData.
// This handler is a no-op for local (non-remote) resources.
type FetchRemoteGraphHandler struct {
	k8sReader         client.Reader
	remoteClientCache *remoteclient.Cache
}

// NewFetchRemoteGraphHandler creates a new FetchRemoteGraphHandler.
func NewFetchRemoteGraphHandler(k8sReader client.Reader, remoteClientCache *remoteclient.Cache) *FetchRemoteGraphHandler {
	return &FetchRemoteGraphHandler{
		k8sReader:         k8sReader,
		remoteClientCache: remoteClientCache,
	}
}

// Handle implements reconciler.Handler.
func (h *FetchRemoteGraphHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, ChainData]) error {
	if !rc.Resource.Spec.IsRemote {
		return nil
	}

	logger := log.FromContext(ctx).WithName("fetch-remote-graph")

	portal := &sreportalv1alpha1.Portal{}
	portalKey := types.NamespacedName{
		Name:      rc.Resource.Spec.PortalRef,
		Namespace: rc.Resource.Namespace,
	}

	if err := h.k8sReader.Get(ctx, portalKey, portal); err != nil {
		return fmt.Errorf("get portal %s: %w", portalKey, err)
	}

	if portal.Spec.Remote == nil {
		return fmt.Errorf("portal %s has no remote configuration but NetworkFlowDiscovery is marked as remote", portalKey.Name)
	}

	remoteClient, err := h.remoteClientFor(ctx, portal)
	if err != nil {
		return fmt.Errorf("build remote client for portal %s: %w", portalKey.Name, err)
	}

	baseURL := portal.Spec.Remote.URL
	logger.V(1).Info("fetching network flows from remote portal", "url", baseURL)

	result, err := remoteClient.FetchNetworkPolicies(ctx, baseURL)
	if err != nil {
		return fmt.Errorf("fetch remote network policies from %s: %w", baseURL, err)
	}

	rc.Data.Nodes = result.Nodes
	rc.Data.Edges = result.Edges
	logger.V(1).Info("fetched remote network flows", "nodes", len(result.Nodes), "edges", len(result.Edges))

	return nil
}

// remoteClientFor returns a cached remoteclient configured with TLS from the Portal spec.
func (h *FetchRemoteGraphHandler) remoteClientFor(ctx context.Context, portal *sreportalv1alpha1.Portal) (*remoteclient.Client, error) {
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
