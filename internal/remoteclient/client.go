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

	// Fetch portal info to get title
	var remoteTitle string
	portalResp, err := portalClient.ListPortals(ctx, connect.NewRequest(&sreportalv1.ListPortalsRequest{}))
	if err == nil {
		// Find the portal with the given name, or use the main portal
		for _, p := range portalResp.Msg.Portals {
			if portalName != "" && p.Name == portalName {
				remoteTitle = p.Title
				break
			}
			if p.Main {
				remoteTitle = p.Title
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
		Groups:      groups,
		RemoteTitle: remoteTitle,
		FQDNCount:   len(resp.Msg.Fqdns),
	}, nil
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
