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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	namespace = "sreportal"

	subsystemController   = "controller"
	subsystemAlertmanager = "alertmanager"
	subsystemRelease      = "release"
	subsystemStatusPage   = "statuspage"
	subsystemHTTP         = "http"
	subsystemMCP          = "mcp"
	subsystemPortal       = "portal"
	subsystemSource       = "source"

	labelPortal     = "portal"
	labelServer     = "server"
	labelSourceType = "source_type"
	labelSource     = "source"
	labelTool       = "tool"
)

// --- Controller metrics ---

var (
	// ReconcileTotal counts reconciliation attempts per controller and result.
	ReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemController,
			Name:      "reconcile_total",
			Help:      "Total number of reconciliations per controller and result (success, error, requeue).",
		},
		[]string{subsystemController, "result"},
	)

	// ReconcileDuration tracks reconciliation duration per controller.
	ReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemController,
			Name:      "reconcile_duration_seconds",
			Help:      "Duration of reconciliation per controller.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{subsystemController},
	)

	// DNSFQDNsTotal tracks the total number of FQDNs across all DNS resources.
	DNSFQDNsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "dns",
			Name:      "fqdns_total",
			Help:      "Total number of FQDNs per portal and source.",
		},
		[]string{labelPortal, labelSource},
	)

	// DNSGroupsTotal tracks the number of DNS groups per portal.
	DNSGroupsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "dns",
			Name:      "groups_total",
			Help:      "Total number of DNS groups per portal.",
		},
		[]string{labelPortal},
	)

	// SourceEndpointsCollected tracks the number of endpoints collected per source type.
	SourceEndpointsCollected = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemSource,
			Name:      "endpoints_collected",
			Help:      "Number of endpoints collected per source type.",
		},
		[]string{labelSourceType},
	)

	// SourceErrorsTotal counts source collection failures per source type.
	SourceErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemSource,
			Name:      "errors_total",
			Help:      "Total number of source collection errors per source type.",
		},
		[]string{labelSourceType},
	)

	// SourceSkippedUpdates counts status updates skipped because the endpoints hash was unchanged.
	SourceSkippedUpdates = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemSource,
			Name:      "skipped_updates_total",
			Help:      "Total number of DNSRecord status updates skipped (endpoints unchanged) per source type.",
		},
		[]string{labelSourceType},
	)

	// AlertsActive tracks the number of active alerts per portal and alertmanager.
	AlertsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemAlertmanager,
			Name:      "alerts_active",
			Help:      "Number of active alerts per portal and alertmanager resource.",
		},
		[]string{labelPortal, "alertmanager"},
	)

	// AlertsFetchErrorsTotal counts alert fetch failures per alertmanager.
	AlertsFetchErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemAlertmanager,
			Name:      "fetch_errors_total",
			Help:      "Total number of alert fetch errors per alertmanager resource.",
		},
		[]string{subsystemAlertmanager},
	)
)

// --- Image inventory metrics ---

var (
	// ImageImagesTotal tracks the number of distinct images per portal and tag type.
	ImageImagesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "image",
			Name:      "images_total",
			Help:      "Number of distinct images per portal and tag type.",
		},
		[]string{labelPortal, "tag_type"},
	)

	// ImageInventorySyncTotal counts ImageInventory sync attempts per resource and result.
	ImageInventorySyncTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "image",
			Name:      "inventory_sync_total",
			Help:      "Total ImageInventory sync attempts by inventory and status (success|error). Counts both local cluster scans and remote-portal fetches.",
		},
		[]string{"inventory", "result"},
	)
)

var (
	// PortalsTotal tracks the total number of portals by type.
	PortalsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemPortal,
			Name:      "total",
			Help:      "Total number of portals by type (local, remote).",
		},
		[]string{"type"},
	)

	// PortalRemoteSyncErrorsTotal counts remote portal sync failures.
	PortalRemoteSyncErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemPortal,
			Name:      "remote_sync_errors_total",
			Help:      "Total number of remote portal sync errors.",
		},
		[]string{labelPortal},
	)

	// PortalRemoteFQDNsSynced tracks the number of FQDNs synced from remote portals.
	PortalRemoteFQDNsSynced = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemPortal,
			Name:      "remote_fqdns_synced",
			Help:      "Number of FQDNs synced from a remote portal.",
		},
		[]string{labelPortal},
	)
)

// --- Release metrics ---

var (
	// ReleaseEntriesTotal tracks the total number of release entries per day CR.
	ReleaseEntriesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemRelease,
			Name:      "entries_total",
			Help:      "Total number of release entries per day CR.",
		},
		[]string{"day"},
	)

	// ReleaseAddTotal counts AddRelease operations.
	ReleaseAddTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemRelease,
			Name:      "add_total",
			Help:      "Total number of AddRelease operations.",
		},
	)

	// ReleaseAddErrorsTotal counts AddRelease errors.
	ReleaseAddErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemRelease,
			Name:      "add_errors_total",
			Help:      "Total number of AddRelease errors.",
		},
	)

	// ReleaseCleanupDeletedTotal counts Release CRs deleted by TTL cleanup.
	ReleaseCleanupDeletedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemRelease,
			Name:      "cleanup_deleted_total",
			Help:      "Total number of Release CRs deleted by TTL cleanup.",
		},
	)
)

// --- Status page metrics ---

var (
	// ComponentsTotal tracks the number of components by portal, group, and status.
	ComponentsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemStatusPage,
			Name:      "components_total",
			Help:      "Number of components by portal, group, and computed status.",
		},
		[]string{labelPortal, "group", "status"},
	)

	// MaintenancesTotal tracks the number of maintenances by portal and phase.
	MaintenancesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemStatusPage,
			Name:      "maintenances_total",
			Help:      "Number of maintenances by portal and phase.",
		},
		[]string{labelPortal, "phase"},
	)

	// IncidentsTotal tracks the number of incidents by portal, phase, and severity.
	IncidentsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemStatusPage,
			Name:      "incidents_total",
			Help:      "Number of incidents by portal, phase, and severity.",
		},
		[]string{labelPortal, "phase", "severity"},
	)
)

// --- HTTP server metrics ---

var (
	// HTTPRequestsTotal counts HTTP requests by method, path, and status code.
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemHTTP,
			Name:      "requests_total",
			Help:      "Total number of HTTP requests by method, handler, and status code.",
		},
		[]string{"method", "handler", "code"},
	)

	// HTTPRequestDuration tracks HTTP request latency by method and handler.
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemHTTP,
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds by method and handler.",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method", "handler"},
	)

	// HTTPRequestsInFlight tracks the number of in-flight HTTP requests.
	HTTPRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemHTTP,
			Name:      "requests_in_flight",
			Help:      "Number of HTTP requests currently being processed.",
		},
	)
)

// --- MCP server metrics ---

var (
	// MCPToolCallsTotal counts MCP tool invocations by server and tool name.
	MCPToolCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemMCP,
			Name:      "tool_calls_total",
			Help:      "Total number of MCP tool calls by server and tool name.",
		},
		[]string{labelServer, labelTool},
	)

	// MCPToolCallDuration tracks MCP tool call duration by server and tool name.
	MCPToolCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemMCP,
			Name:      "tool_call_duration_seconds",
			Help:      "Duration of MCP tool calls by server and tool name.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{labelServer, labelTool},
	)

	// MCPToolCallErrorsTotal counts MCP tool call errors by server and tool name.
	MCPToolCallErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemMCP,
			Name:      "tool_call_errors_total",
			Help:      "Total number of MCP tool call errors by server and tool name.",
		},
		[]string{labelServer, labelTool},
	)

	// MCPSessionsActive tracks the number of active MCP sessions per server.
	MCPSessionsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemMCP,
			Name:      "sessions_active",
			Help:      "Number of active MCP sessions per server.",
		},
		[]string{labelServer},
	)
)

func init() {
	// Register all metrics with the controller-runtime metrics registry
	// so they are exposed on the /metrics endpoint.
	metrics.Registry.MustRegister(
		// Controller
		ReconcileTotal,
		ReconcileDuration,
		// DNS
		DNSFQDNsTotal,
		DNSGroupsTotal,
		// Source
		SourceEndpointsCollected,
		SourceErrorsTotal,
		SourceSkippedUpdates,
		// Alertmanager
		AlertsActive,
		AlertsFetchErrorsTotal,
		// Image
		ImageImagesTotal,
		ImageInventorySyncTotal,
		// Portal
		PortalsTotal,
		PortalRemoteSyncErrorsTotal,
		PortalRemoteFQDNsSynced,
		// Release
		ReleaseEntriesTotal,
		ReleaseAddTotal,
		ReleaseAddErrorsTotal,
		ReleaseCleanupDeletedTotal,
		// Status page
		ComponentsTotal,
		MaintenancesTotal,
		IncidentsTotal,
		// HTTP
		HTTPRequestsTotal,
		HTTPRequestDuration,
		HTTPRequestsInFlight,
		// MCP
		MCPToolCallsTotal,
		MCPToolCallDuration,
		MCPToolCallErrorsTotal,
		MCPSessionsActive,
	)
}
