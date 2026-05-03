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
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
	"github.com/golgoth31/sreportal/internal/tlsutil"
)

// DefaultRemoteSyncInterval is the default interval for syncing remote portals.
const DefaultRemoteSyncInterval = 5 * time.Minute

// BuildRemoteClientHandler builds a cached remote client for remote portals.
// No-op for local portals.
type BuildRemoteClientHandler struct {
	client client.Client
	cache  *remoteclient.Cache
}

// NewBuildRemoteClientHandler creates a new BuildRemoteClientHandler.
func NewBuildRemoteClientHandler(c client.Client, cache *remoteclient.Cache) *BuildRemoteClientHandler {
	return &BuildRemoteClientHandler{client: c, cache: cache}
}

// Handle implements reconciler.Handler.
func (h *BuildRemoteClientHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	portal := rc.Resource
	if portal.Spec.Remote == nil {
		return nil
	}

	logger := log.FromContext(ctx).WithName("build-remote-client")

	remoteClient, err := h.remoteClientFor(ctx, portal)
	if err != nil {
		logger.Error(err, "failed to build remote client", "url", portal.Spec.Remote.URL)

		base := portal.DeepCopy()
		portal.Status.Ready = false
		if portal.Status.RemoteSync == nil {
			portal.Status.RemoteSync = &sreportalv1alpha1.RemoteSyncStatus{}
		}
		portal.Status.RemoteSync.LastSyncError = err.Error()

		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               conditionTypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             "TLSConfigFailed",
			Message:            "Failed to build TLS configuration: " + err.Error(),
			LastTransitionTime: metav1.Now(),
		})

		if patchErr := h.client.Status().Patch(ctx, portal, client.MergeFrom(base)); patchErr != nil {
			return fmt.Errorf("patch Portal status: %w", patchErr)
		}

		rc.Result = ctrl.Result{RequeueAfter: DefaultRemoteSyncInterval}
		return nil
	}

	rc.Data.RemoteClient = remoteClient
	return nil
}

// remoteClientFor returns a cached remote client for the given portal.
func (h *BuildRemoteClientHandler) remoteClientFor(ctx context.Context, portal *sreportalv1alpha1.Portal) (*remoteclient.Client, error) {
	remote := portal.Spec.Remote
	if remote.TLS == nil {
		return h.cache.Fallback(), nil
	}

	key := portal.Namespace + "/" + portal.Name
	versions, err := tlsutil.SecretVersions(ctx, h.client, portal.Namespace, remote.TLS)
	if err != nil {
		return nil, fmt.Errorf("read TLS secret versions: %w", err)
	}

	if cached := h.cache.Get(key, versions); cached != nil {
		return cached, nil
	}

	tlsConfig, err := tlsutil.BuildTLSConfig(ctx, h.client, portal.Namespace, remote.TLS)
	if err != nil {
		return nil, fmt.Errorf("build TLS config: %w", err)
	}

	c := remoteclient.NewClient(remoteclient.WithTLSConfig(tlsConfig))
	h.cache.Put(key, versions, c)

	return c, nil
}
