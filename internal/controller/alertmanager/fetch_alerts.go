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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanager"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
	"github.com/golgoth31/sreportal/internal/tlsutil"
)

const (
	// DataKeyAlerts is the key for storing fetched alerts in ReconcileContext.Data.
	DataKeyAlerts = "alerts"

	// DataKeySilences is the key for storing fetched silences in ReconcileContext.Data.
	DataKeySilences = "silences"

	// LabelRemoteAlertmanagerName is the label key for identifying the remote alertmanager name.
	LabelRemoteAlertmanagerName = "sreportal.io/remote-alertmanager-name"
)

// FetchAlertsHandler retrieves active alerts from either the local Alertmanager API
// or a remote SRE Portal, depending on the IsRemote flag on the resource spec.
// For local, uses DataFetcher when available to get alerts with receivers and silences.
// For remote portals, it looks up the Portal CR to obtain TLS configuration and
// builds a TLS-aware remoteclient — the same approach as the Portal controller.
type FetchAlertsHandler struct {
	localDataFetcher domainalertmanager.DataFetcher
	localFetcher     domainalertmanager.Fetcher
	k8sReader        client.Reader
}

// NewFetchAlertsHandler creates a new FetchAlertsHandler.
// localDataFetcher may be nil; then localFetcher is used for basic alerts only.
func NewFetchAlertsHandler(
	localDataFetcher domainalertmanager.DataFetcher,
	localFetcher domainalertmanager.Fetcher,
	k8sReader client.Reader,
) *FetchAlertsHandler {
	return &FetchAlertsHandler{
		localDataFetcher: localDataFetcher,
		localFetcher:     localFetcher,
		k8sReader:        k8sReader,
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

	if h.localDataFetcher != nil {
		log.V(1).Info("fetching alertmanager data (alerts+receivers+silences) from local", "url", localURL)
		data, err := h.localDataFetcher.GetAlertmanagerData(ctx, localURL)
		if err != nil {
			return fmt.Errorf("fetch alertmanager data from %s: %w", localURL, err)
		}
		log.V(1).Info("fetched alertmanager data", "alerts", len(data.Alerts), "silences", len(data.Silences))
		rc.Data[DataKeyAlerts] = data.Alerts
		rc.Data[DataKeySilences] = data.Silences
		return nil
	}

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
	// Look up the Portal CR to get the remote URL and TLS configuration.
	portal := &sreportalv1alpha1.Portal{}
	portalKey := types.NamespacedName{
		Name:      rc.Resource.Spec.PortalRef,
		Namespace: rc.Resource.Namespace,
	}

	if err := h.k8sReader.Get(ctx, portalKey, portal); err != nil {
		return fmt.Errorf("get portal %s: %w", portalKey, err)
	}

	if portal.Spec.Remote == nil {
		return fmt.Errorf("portal %s has no remote configuration but alertmanager is marked as remote", portalKey.Name)
	}

	// Build a TLS-aware remote client, the same way the Portal controller does.
	remoteClient, err := h.remoteClientFor(ctx, portal)
	if err != nil {
		return fmt.Errorf("build remote client for portal %s: %w", portalKey.Name, err)
	}

	baseURL := portal.Spec.Remote.URL
	portalName := portal.Spec.Remote.Portal

	// Each local remote CR represents one specific alertmanager on the remote portal.
	// The label identifies which remote alertmanager this CR corresponds to.
	remoteAMName := rc.Resource.Labels[LabelRemoteAlertmanagerName]
	log.V(1).Info("fetching active alerts from remote portal", "url", baseURL, "portalName", portalName, "remoteAlertmanager", remoteAMName)

	result, err := remoteClient.FetchAlerts(ctx, baseURL, portalName, remoteAMName)
	if err != nil {
		return fmt.Errorf("fetch remote alerts from %s: %w", baseURL, err)
	}

	alerts := toAlertsDomain(result.Alerts)
	silences := toSilencesDomain(result.Silences)
	log.V(1).Info("fetched remote alerts", "alerts", len(alerts), "silences", len(silences))
	rc.Data[DataKeyAlerts] = alerts
	rc.Data[DataKeySilences] = silences

	return nil
}

// remoteClientFor returns a remoteclient configured with TLS from the Portal spec.
func (h *FetchAlertsHandler) remoteClientFor(ctx context.Context, portal *sreportalv1alpha1.Portal) (*remoteclient.Client, error) {
	if portal.Spec.Remote.TLS == nil {
		return remoteclient.NewClient(), nil
	}

	tlsConfig, err := tlsutil.BuildTLSConfig(ctx, h.k8sReader, portal.Namespace, portal.Spec.Remote.TLS)
	if err != nil {
		return nil, fmt.Errorf("build TLS config: %w", err)
	}

	return remoteclient.NewClient(remoteclient.WithTLSConfig(tlsConfig)), nil
}

// toAlertsDomain converts CRD AlertStatus values to domain Alert objects.
func toAlertsDomain(statuses []sreportalv1alpha1.AlertStatus) []domainalertmanager.Alert {
	alerts := make([]domainalertmanager.Alert, 0, len(statuses))

	for _, a := range statuses {
		receivers := make([]domainalertmanager.Receiver, 0, len(a.Receivers))
		for _, name := range a.Receivers {
			r, err := domainalertmanager.NewReceiver(name)
			if err == nil {
				receivers = append(receivers, r)
			}
		}

		da := domainalertmanager.Alert{
			Fingerprint: a.Fingerprint,
			Labels:      a.Labels,
			Annotations: a.Annotations,
			State:       domainalertmanager.State(a.State),
			StartsAt:    a.StartsAt.Time,
			UpdatedAt:   a.UpdatedAt.Time,
			Receivers:   receivers,
			SilencedBy:  a.SilencedBy,
		}
		if a.EndsAt != nil {
			endsAt := a.EndsAt.Time
			da.EndsAt = &endsAt
		}

		alerts = append(alerts, da)
	}

	return alerts
}

// toSilencesDomain converts CRD SilenceStatus values to domain Silence objects.
func toSilencesDomain(statuses []sreportalv1alpha1.SilenceStatus) []domainalertmanager.Silence {
	silences := make([]domainalertmanager.Silence, 0, len(statuses))

	for _, s := range statuses {
		matchers := make([]domainalertmanager.Matcher, 0, len(s.Matchers))
		for _, m := range s.Matchers {
			matchers = append(matchers, domainalertmanager.Matcher{
				Name:    m.Name,
				Value:   m.Value,
				IsRegex: m.IsRegex,
			})
		}

		silence, err := domainalertmanager.NewSilence(
			s.ID,
			matchers,
			s.StartsAt.Time,
			s.EndsAt.Time,
			domainalertmanager.SilenceStatus(s.Status),
			s.CreatedBy,
			s.Comment,
			s.UpdatedAt.Time,
		)
		if err == nil {
			silences = append(silences, silence)
		}
	}

	return silences
}
