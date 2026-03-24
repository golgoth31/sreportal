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

package networkflowdiscovery

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// EnsureNFDRunnable creates a manager.Runnable that ensures a NetworkFlowDiscovery
// resource exists for the main portal at startup.
type EnsureNFDRunnable struct {
	client      client.Client
	cacheReader cache.Cache
	namespace   string
	portalRef   string
}

// NewEnsureNFDRunnable creates a new EnsureNFDRunnable.
func NewEnsureNFDRunnable(c client.Client, cacheReader cache.Cache, namespace, portalRef string) *EnsureNFDRunnable {
	return &EnsureNFDRunnable{
		client:      c,
		cacheReader: cacheReader,
		namespace:   namespace,
		portalRef:   portalRef,
	}
}

// Start implements manager.Runnable. It runs once at startup.
func (r *EnsureNFDRunnable) Start(ctx context.Context) error {
	log := ctrl.Log.WithName("ensure-nfd")

	log.Info("waiting for cache to sync")
	if !r.cacheReader.WaitForCacheSync(ctx) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := fmt.Errorf("cache sync failed - ensure CRDs are installed (run: make install)")
		log.Error(err, "failed to wait for cache sync")
		return err
	}
	log.Info("cache synced successfully")

	// Check if a NetworkFlowDiscovery already exists for this portal
	nfdName := "netflow-" + r.portalRef
	var existing sreportalv1alpha1.NetworkFlowDiscovery
	err := r.client.Get(ctx, client.ObjectKey{Name: nfdName, Namespace: r.namespace}, &existing)
	if err == nil {
		log.Info("NetworkFlowDiscovery already exists", "name", nfdName, "namespace", r.namespace)
		return nil
	}
	if !errors.IsNotFound(err) {
		log.Error(err, "failed to get NetworkFlowDiscovery")
		return err
	}

	// Create it
	nfd := &sreportalv1alpha1.NetworkFlowDiscovery{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nfdName,
			Namespace: r.namespace,
		},
		Spec: sreportalv1alpha1.NetworkFlowDiscoverySpec{
			PortalRef: r.portalRef,
		},
	}

	if err := r.client.Create(ctx, nfd); err != nil {
		log.Error(err, "failed to create NetworkFlowDiscovery")
		return err
	}

	log.Info("created NetworkFlowDiscovery", "name", nfdName, "namespace", r.namespace, "portalRef", r.portalRef)
	return nil
}

// NeedLeaderElection returns true so this only runs on the leader.
func (r *EnsureNFDRunnable) NeedLeaderElection() bool {
	return true
}
