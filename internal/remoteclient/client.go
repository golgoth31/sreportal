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

// Package remoteclient provides a client for communicating with remote SRE Portal instances.
package remoteclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sort"
	"time"

	"connectrpc.com/connect"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	sreportalv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// DefaultTimeout is the default timeout for remote portal requests.
const DefaultTimeout = 10 * time.Second

// DefaultRetryAttempts is the default number of retry attempts.
const DefaultRetryAttempts = 3

// DefaultRetryDelay is the initial delay between retries.
const DefaultRetryDelay = 500 * time.Millisecond

// Client provides methods to communicate with remote SRE Portal instances.
type Client struct {
	httpClient    *http.Client
	timeout       time.Duration
	retryAttempts int
	retryDelay    time.Duration
}

// Option is a function that configures the Client.
type Option func(*Client)

// WithTimeout sets the timeout for requests.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithRetryAttempts sets the number of retry attempts.
func WithRetryAttempts(attempts int) Option {
	return func(c *Client) {
		c.retryAttempts = attempts
	}
}

// WithRetryDelay sets the initial delay between retries.
func WithRetryDelay(delay time.Duration) Option {
	return func(c *Client) {
		c.retryDelay = delay
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithTLSConfig sets a custom TLS configuration for the HTTP client.
func WithTLSConfig(tlsConfig *tls.Config) Option {
	return func(c *Client) {
		c.httpClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}
}

// NewClient creates a new remote portal client with the given options.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		timeout:       DefaultTimeout,
		retryAttempts: DefaultRetryAttempts,
		retryDelay:    DefaultRetryDelay,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Ensure HTTP client timeout matches configured timeout
	c.httpClient.Timeout = c.timeout

	return c
}

// FetchResult contains the result of fetching data from a remote portal.
type FetchResult struct {
	// Groups contains the FQDN groups fetched from the remote portal.
	Groups []sreportalv1alpha1.FQDNGroupStatus
	// RemoteTitle is the title of the remote portal.
	RemoteTitle string
	// FQDNCount is the total number of FQDNs fetched.
	FQDNCount int
	// RemoteFeatures contains the feature flags reported by the remote portal.
	RemoteFeatures *sreportalv1alpha1.PortalFeaturesStatus
}

// FetchFQDNs fetches FQDNs from a remote portal.
// The portalName parameter is used to filter FQDNs by portal on the remote side.
func (c *Client) FetchFQDNs(ctx context.Context, baseURL string, portalName string) (*FetchResult, error) {
	var lastErr error

	for attempt := 0; attempt < c.retryAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := c.retryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := c.doFetchFQDNs(ctx, baseURL, portalName)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", c.retryAttempts, lastErr)
}

func (c *Client) doFetchFQDNs(ctx context.Context, baseURL string, portalName string) (*FetchResult, error) {
	// Create DNS service client
	dnsClient := sreportalv1connect.NewDNSServiceClient(
		c.httpClient,
		baseURL,
	)

	// Create portal service client to get portal info
	portalClient := sreportalv1connect.NewPortalServiceClient(
		c.httpClient,
		baseURL,
	)

	// Fetch portal info to get title and features
	var remoteTitle string
	var remoteFeatures *sreportalv1alpha1.PortalFeaturesStatus
	portalResp, err := portalClient.ListPortals(ctx, connect.NewRequest(&sreportalv1.ListPortalsRequest{}))
	if err == nil {
		// Find the portal with the given name, or use the main portal
		var matched *sreportalv1.Portal
		for _, p := range portalResp.Msg.Portals {
			if portalName != "" && p.Name == portalName {
				matched = p
				break
			}
			if p.Main {
				matched = p
			}
		}
		if matched != nil {
			remoteTitle = matched.Title
			if matched.Features != nil {
				remoteFeatures = &sreportalv1alpha1.PortalFeaturesStatus{
					DNS:           matched.Features.Dns,
					Releases:      matched.Features.Releases,
					NetworkPolicy: matched.Features.NetworkPolicy,
					Alerts:        matched.Features.Alerts,
					StatusPage:    matched.Features.StatusPage,
				}
			}
		}
	}

	// Fetch FQDNs
	req := connect.NewRequest(&sreportalv1.ListFQDNsRequest{
		Portal: portalName,
	})

	resp, err := dnsClient.ListFQDNs(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch FQDNs from remote portal: %w", err)
	}

	// Convert to FQDNGroupStatus format
	groups := convertToGroups(resp.Msg.Fqdns)

	return &FetchResult{
		Groups:         groups,
		RemoteTitle:    remoteTitle,
		FQDNCount:      len(resp.Msg.Fqdns),
		RemoteFeatures: remoteFeatures,
	}, nil
}

// AlertsFetchResult contains the result of fetching alerts from a remote portal.
type AlertsFetchResult struct {
	// Alerts contains the active alerts fetched from the remote portal.
	Alerts []sreportalv1alpha1.AlertStatus
	// Silences contains the silences fetched from the remote portal.
	Silences []sreportalv1alpha1.SilenceStatus
}

// RemoteAlertmanagerInfo describes an alertmanager discovered on a remote portal.
type RemoteAlertmanagerInfo struct {
	// Name is the alertmanager resource name on the remote portal.
	Name string
	// Namespace is the alertmanager resource namespace on the remote portal.
	Namespace string
	// RemoteURL is the externally-reachable Alertmanager URL from the remote resource.
	RemoteURL string
	// LocalURL is the cluster-internal Alertmanager URL from the remote resource.
	LocalURL string
}

// DiscoverAlertmanagers lists alertmanager resources available on a remote portal.
// Used by the Portal controller to create one local CR per remote alertmanager.
func (c *Client) DiscoverAlertmanagers(ctx context.Context, baseURL string, portalName string) ([]RemoteAlertmanagerInfo, error) {
	var lastErr error

	for attempt := 0; attempt < c.retryAttempts; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := c.doDiscoverAlertmanagers(ctx, baseURL, portalName)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("discover alertmanagers failed after %d attempts: %w", c.retryAttempts, lastErr)
}

func (c *Client) doDiscoverAlertmanagers(ctx context.Context, baseURL string, portalName string) ([]RemoteAlertmanagerInfo, error) {
	alertsClient := sreportalv1connect.NewAlertmanagerServiceClient(
		c.httpClient,
		baseURL,
	)

	resp, err := alertsClient.ListAlerts(ctx, connect.NewRequest(&sreportalv1.ListAlertsRequest{
		Portal: portalName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list alertmanagers from remote portal: %w", err)
	}

	infos := make([]RemoteAlertmanagerInfo, 0, len(resp.Msg.Alertmanagers))
	for _, resource := range resp.Msg.Alertmanagers {
		infos = append(infos, RemoteAlertmanagerInfo{
			Name:      resource.Name,
			Namespace: resource.Namespace,
			RemoteURL: resource.RemoteUrl,
			LocalURL:  resource.LocalUrl,
		})
	}

	return infos, nil
}

// FetchAlerts fetches active alerts from a remote portal via the AlertmanagerService Connect API.
// The portalName parameter filters alerts by portal on the remote side.
// The alertmanagerName parameter filters for a specific alertmanager resource; if empty, all alerts are returned.
func (c *Client) FetchAlerts(ctx context.Context, baseURL string, portalName string, alertmanagerName string) (*AlertsFetchResult, error) {
	var lastErr error

	for attempt := 0; attempt < c.retryAttempts; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := c.doFetchAlerts(ctx, baseURL, portalName, alertmanagerName)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("fetch alerts failed after %d attempts: %w", c.retryAttempts, lastErr)
}

func (c *Client) doFetchAlerts(ctx context.Context, baseURL string, portalName string, alertmanagerName string) (*AlertsFetchResult, error) {
	alertsClient := sreportalv1connect.NewAlertmanagerServiceClient(
		c.httpClient,
		baseURL,
	)

	resp, err := alertsClient.ListAlerts(ctx, connect.NewRequest(&sreportalv1.ListAlertsRequest{
		Portal: portalName,
	}))
	if err != nil {
		return nil, fmt.Errorf("fetch alerts from remote portal: %w", err)
	}

	var allAlerts []sreportalv1alpha1.AlertStatus
	var allSilences []sreportalv1alpha1.SilenceStatus

	for _, resource := range resp.Msg.Alertmanagers {
		// Filter by alertmanager name if specified.
		if alertmanagerName != "" && resource.Name != alertmanagerName {
			continue
		}

		for _, a := range resource.Alerts {
			alert := sreportalv1alpha1.AlertStatus{
				Fingerprint: a.Fingerprint,
				Labels:      a.Labels,
				Annotations: a.Annotations,
				State:       a.State,
				Receivers:   a.Receivers,
				SilencedBy:  a.SilencedBy,
			}
			if a.StartsAt != nil {
				alert.StartsAt = metav1.NewTime(a.StartsAt.AsTime())
			}
			if a.UpdatedAt != nil {
				alert.UpdatedAt = metav1.NewTime(a.UpdatedAt.AsTime())
			}
			if a.EndsAt != nil {
				endsAt := metav1.NewTime(a.EndsAt.AsTime())
				alert.EndsAt = &endsAt
			}
			allAlerts = append(allAlerts, alert)
		}

		for _, s := range resource.Silences {
			matchers := make([]sreportalv1alpha1.MatcherStatus, 0, len(s.Matchers))
			for _, m := range s.Matchers {
				matchers = append(matchers, sreportalv1alpha1.MatcherStatus{
					Name:    m.Name,
					Value:   m.Value,
					IsRegex: m.IsRegex,
				})
			}
			silence := sreportalv1alpha1.SilenceStatus{
				ID:        s.Id,
				Matchers:  matchers,
				Status:    s.Status,
				CreatedBy: s.CreatedBy,
				Comment:   s.Comment,
			}
			if s.StartsAt != nil {
				silence.StartsAt = metav1.NewTime(s.StartsAt.AsTime())
			}
			if s.EndsAt != nil {
				silence.EndsAt = metav1.NewTime(s.EndsAt.AsTime())
			}
			if s.UpdatedAt != nil {
				silence.UpdatedAt = metav1.NewTime(s.UpdatedAt.AsTime())
			}
			allSilences = append(allSilences, silence)
		}
	}

	return &AlertsFetchResult{
		Alerts:   allAlerts,
		Silences: allSilences,
	}, nil
}

// NetworkFlowsFetchResult contains the result of fetching network flows from a remote portal.
type NetworkFlowsFetchResult struct {
	// Nodes contains the flow nodes fetched from the remote portal.
	Nodes []sreportalv1alpha1.FlowNode
	// Edges contains the flow edges fetched from the remote portal.
	Edges []sreportalv1alpha1.FlowEdge
}

// FetchNetworkPolicies fetches network flow nodes and edges from a remote portal.
func (c *Client) FetchNetworkPolicies(ctx context.Context, baseURL string) (*NetworkFlowsFetchResult, error) {
	var lastErr error

	for attempt := 0; attempt < c.retryAttempts; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := c.doFetchNetworkPolicies(ctx, baseURL)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("fetch network policies failed after %d attempts: %w", c.retryAttempts, lastErr)
}

func (c *Client) doFetchNetworkPolicies(ctx context.Context, baseURL string) (*NetworkFlowsFetchResult, error) {
	netpolClient := sreportalv1connect.NewNetworkPolicyServiceClient(
		c.httpClient,
		baseURL,
	)

	resp, err := netpolClient.ListNetworkPolicies(ctx, connect.NewRequest(&sreportalv1.ListNetworkPoliciesRequest{}))
	if err != nil {
		return nil, fmt.Errorf("fetch network policies from remote portal: %w", err)
	}

	nodes := make([]sreportalv1alpha1.FlowNode, 0, len(resp.Msg.Nodes))
	for _, n := range resp.Msg.Nodes {
		nodes = append(nodes, sreportalv1alpha1.FlowNode{
			ID:        n.Id,
			Label:     n.Label,
			Namespace: n.Namespace,
			NodeType:  n.NodeType,
			Group:     n.Group,
		})
	}

	edges := make([]sreportalv1alpha1.FlowEdge, 0, len(resp.Msg.Edges))
	for _, e := range resp.Msg.Edges {
		edges = append(edges, sreportalv1alpha1.FlowEdge{
			From:      e.From,
			To:        e.To,
			EdgeType:  e.EdgeType,
			Used:      e.Used,
			Evaluated: e.Evaluated,
		})
	}

	return &NetworkFlowsFetchResult{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// RemoteImage is a domain-friendly view of a single image entry returned by
// a remote portal's ImageService. It exists so chain handlers don't need to
// import the proto package directly.
type RemoteImage struct {
	Registry   string
	Repository string
	Tag        string
	TagType    string
	Workloads  []RemoteImageWorkload
}

// RemoteImageWorkload describes a workload using a remote image.
type RemoteImageWorkload struct {
	Kind      string
	Namespace string
	Name      string
	Container string
	Source    string
}

// ImagesFetchResult contains the result of fetching images from a remote portal.
type ImagesFetchResult struct {
	// Images contains the image entries returned by the remote ImageService.
	Images []*RemoteImage
}

// FetchImages fetches the image inventory of a remote portal via the
// ImageService Connect API. The portalName parameter filters images by portal
// on the remote side.
func (c *Client) FetchImages(ctx context.Context, baseURL string, portalName string) (*ImagesFetchResult, error) {
	var lastErr error

	for attempt := 0; attempt < c.retryAttempts; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := c.doFetchImages(ctx, baseURL, portalName)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("fetch images failed after %d attempts: %w", c.retryAttempts, lastErr)
}

func (c *Client) doFetchImages(ctx context.Context, baseURL string, portalName string) (*ImagesFetchResult, error) {
	imageClient := sreportalv1connect.NewImageServiceClient(
		c.httpClient,
		baseURL,
	)

	resp, err := imageClient.ListImages(ctx, connect.NewRequest(&sreportalv1.ListImagesRequest{
		Portal: portalName,
	}))
	if err != nil {
		return nil, fmt.Errorf("fetch images from remote portal: %w", err)
	}

	images := make([]*RemoteImage, 0, len(resp.Msg.Images))
	for _, in := range resp.Msg.Images {
		workloads := make([]RemoteImageWorkload, 0, len(in.Workloads))
		for _, w := range in.Workloads {
			workloads = append(workloads, RemoteImageWorkload{
				Kind:      w.Kind,
				Namespace: w.Namespace,
				Name:      w.Name,
				Container: w.Container,
				Source:    w.Source,
			})
		}
		images = append(images, &RemoteImage{
			Registry:   in.Registry,
			Repository: in.Repository,
			Tag:        in.Tag,
			TagType:    in.TagType,
			Workloads:  workloads,
		})
	}

	return &ImagesFetchResult{Images: images}, nil
}

// HealthCheck performs a health check on a remote portal by attempting to list portals.
func (c *Client) HealthCheck(ctx context.Context, baseURL string) error {
	portalClient := sreportalv1connect.NewPortalServiceClient(
		c.httpClient,
		baseURL,
	)

	_, err := portalClient.ListPortals(ctx, connect.NewRequest(&sreportalv1.ListPortalsRequest{}))
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	return nil
}

// convertToGroups converts a list of FQDNs from the proto format to the CRD status format.
func convertToGroups(fqdns []*sreportalv1.FQDN) []sreportalv1alpha1.FQDNGroupStatus {
	// Group FQDNs by group name
	groupMap := make(map[string]*sreportalv1alpha1.FQDNGroupStatus)

	for _, fqdn := range fqdns {
		groupNames := fqdn.Groups
		if len(groupNames) == 0 {
			groupNames = []string{"default"}
		}

		var lastSeen time.Time
		if fqdn.LastSeen != nil {
			lastSeen = fqdn.LastSeen.AsTime()
		} else {
			lastSeen = time.Now()
		}

		fqdnStatus := sreportalv1alpha1.FQDNStatus{
			FQDN:        fqdn.Name,
			Description: fqdn.Description,
			RecordType:  fqdn.RecordType,
			Targets:     fqdn.Targets,
			LastSeen:    metav1.Time{Time: lastSeen},
			SyncStatus:  fqdn.SyncStatus,
		}

		for _, groupName := range groupNames {
			group, exists := groupMap[groupName]
			if !exists {
				group = &sreportalv1alpha1.FQDNGroupStatus{
					Name:   groupName,
					Source: "remote",
					FQDNs:  []sreportalv1alpha1.FQDNStatus{},
				}
				groupMap[groupName] = group
			}

			group.FQDNs = append(group.FQDNs, fqdnStatus)
		}
	}

	// Convert map to sorted slice (map iteration is non-deterministic)
	groups := make([]sreportalv1alpha1.FQDNGroupStatus, 0, len(groupMap))
	for _, group := range groupMap {
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups
}
