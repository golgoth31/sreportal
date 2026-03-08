// Package alertmanagerclient provides HTTP clients for fetching alerts.
package alertmanagerclient

import (
	"context"
	"fmt"

	"github.com/golgoth31/sreportal/internal/domain/alertmanager"
	"github.com/golgoth31/sreportal/internal/remoteclient"
)

// RemoteFetcher fetches alerts from a remote SRE Portal via the Connect API.
// It implements domainalertmanager.Fetcher by translating the remote portal's
// AlertStatus CRD format back into domain Alert objects.
type RemoteFetcher struct {
	remoteClient *remoteclient.Client
}

// NewRemoteFetcher creates a new RemoteFetcher.
func NewRemoteFetcher(rc *remoteclient.Client) *RemoteFetcher {
	return &RemoteFetcher{remoteClient: rc}
}

// GetActiveAlerts fetches alerts from a remote portal.
// The baseURL is expected to contain the remote portal base URL and the portal name
// separated by a pipe character: "https://remote:8080|portalName".
// This convention is used because the Fetcher interface only accepts a single URL string.
func (f *RemoteFetcher) GetActiveAlerts(ctx context.Context, baseURL string) ([]alertmanager.Alert, error) {
	remoteURL, portalName := parseRemoteURL(baseURL)

	result, err := f.remoteClient.FetchAlerts(ctx, remoteURL, portalName)
	if err != nil {
		return nil, fmt.Errorf("%w: remote portal: %v", alertmanager.ErrFetchAlerts, err)
	}

	alerts := make([]alertmanager.Alert, 0, len(result.Alerts))
	for _, a := range result.Alerts {
		da := alertmanager.Alert{
			Fingerprint: a.Fingerprint,
			Labels:      a.Labels,
			Annotations: a.Annotations,
			State:       alertmanager.State(a.State),
			StartsAt:    a.StartsAt.Time,
			UpdatedAt:   a.UpdatedAt.Time,
		}
		if a.EndsAt != nil {
			endsAt := a.EndsAt.Time
			da.EndsAt = &endsAt
		}
		alerts = append(alerts, da)
	}

	return alerts, nil
}

// parseRemoteURL splits a combined "baseURL|portalName" string.
func parseRemoteURL(combined string) (baseURL, portalName string) {
	for i := len(combined) - 1; i >= 0; i-- {
		if combined[i] == '|' {
			return combined[:i], combined[i+1:]
		}
	}
	return combined, ""
}
