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

package alertmanager

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanager"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// DataKeyAlerts is the key for storing fetched alerts in ReconcileContext.Data.
	DataKeyAlerts = "alerts"
)

// FetchAlertsHandler retrieves active alerts from either the local Alertmanager API
// or a remote SRE Portal, depending on the IsRemote flag on the resource spec.
type FetchAlertsHandler struct {
	localFetcher  domainalertmanager.Fetcher
	remoteFetcher domainalertmanager.Fetcher
}

// NewFetchAlertsHandler creates a new FetchAlertsHandler.
func NewFetchAlertsHandler(localFetcher domainalertmanager.Fetcher, remoteFetcher domainalertmanager.Fetcher) *FetchAlertsHandler {
	return &FetchAlertsHandler{
		localFetcher:  localFetcher,
		remoteFetcher: remoteFetcher,
	}
}

// Handle implements reconciler.Handler.
func (h *FetchAlertsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Alertmanager]) error {
	log := logf.FromContext(ctx).WithName("fetch-alerts")

	if rc.Resource.Spec.IsRemote {
		return h.handleRemote(ctx, rc, log)
	}

	return h.handleLocal(ctx, rc, log)
}

func (h *FetchAlertsHandler) handleLocal(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Alertmanager], log logr.Logger) error {
	localURL := rc.Resource.Spec.URL.Local
	log.V(1).Info("fetching active alerts from local Alertmanager", "url", localURL)

	alerts, err := h.localFetcher.GetActiveAlerts(ctx, localURL)
	if err != nil {
		return fmt.Errorf("fetch alerts from %s: %w", localURL, err)
	}

	log.V(1).Info("fetched alerts", "count", len(alerts))
	rc.Data[DataKeyAlerts] = alerts

	return nil
}

func (h *FetchAlertsHandler) handleRemote(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Alertmanager], log logr.Logger) error {
	// For remote portals, URL.Local contains the combined "baseURL|portalName" string
	remoteURL := rc.Resource.Spec.URL.Local
	log.V(1).Info("fetching active alerts from remote portal", "url", remoteURL)

	alerts, err := h.remoteFetcher.GetActiveAlerts(ctx, remoteURL)
	if err != nil {
		return fmt.Errorf("fetch remote alerts from %s: %w", remoteURL, err)
	}

	log.V(1).Info("fetched remote alerts", "count", len(alerts))
	rc.Data[DataKeyAlerts] = alerts

	return nil
}
