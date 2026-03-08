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

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanager"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// DataKeyAlerts is the key for storing fetched alerts in ReconcileContext.Data.
	DataKeyAlerts = "alerts"
)

// FetchAlertsHandler retrieves active alerts from the Alertmanager API
// using the local URL defined in the resource spec.
type FetchAlertsHandler struct {
	fetcher domainalertmanager.Fetcher
}

// NewFetchAlertsHandler creates a new FetchAlertsHandler.
func NewFetchAlertsHandler(f domainalertmanager.Fetcher) *FetchAlertsHandler {
	return &FetchAlertsHandler{fetcher: f}
}

// Handle implements reconciler.Handler.
func (h *FetchAlertsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Alertmanager]) error {
	log := logf.FromContext(ctx).WithName("fetch-alerts")

	localURL := rc.Resource.Spec.URL.Local
	log.V(1).Info("fetching active alerts", "url", localURL)

	alerts, err := h.fetcher.GetActiveAlerts(ctx, localURL)
	if err != nil {
		return fmt.Errorf("fetch alerts from %s: %w", localURL, err)
	}

	log.V(1).Info("fetched alerts", "count", len(alerts))
	rc.Data[DataKeyAlerts] = alerts

	return nil
}
