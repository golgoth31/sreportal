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

	subsystemController    = "controller"
	subsystemAlertmanager  = "alertmanager"
	subsystemRelease       = "release"
	subsystemStatusPage    = "statuspage"
	subsystemHTTP          = "http"
	subsystemMCP           = "mcp"
	subsystemPortal        = "portal"
	subsystemSource        = "source"
	subsystemImageRegistry = "imageregistry"
	subsystemDNS           = "dns"

	labelKind       = "kind"
	labelName       = "name"
	labelPortal     = "portal"
	labelServer     = "server"
	labelSourceType = "source_type"
	labelSource     = "source"
	labelTool       = "tool"
	labelHost       = "host"
	labelNamespace  = "namespace"
	labelResult     = "result"
	labelHandler    = "handler"
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

	// ReconcileDuration tracks reconciliation duration per controller and handler.
	// The handler label is empty ("") for the overall reconcile duration; for
	// chain-based controllers, it is set to the handler's Go type name (e.g.
	// "ScanWorkloadsHandler") for each step inside reconciler.Chain.Execute.
	ReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemController,
			Name:      "reconcile_duration_seconds",
			Help:      "Duration of reconciliation per controller and chain handler. handler=\"\" is the overall reconcile; handler=\"<TypeName>\" is a single chain step.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{subsystemController, labelHandler},
	)

	// ReadstoreWriterErrors counts errors when projecting reconciled state into
	// in-memory read stores. These errors do not fail the reconcile, but are
	// recorded so operators can spot drift between CRD state and the read path
	// served to gRPC/MCP.
	ReadstoreWriterErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemController,
			Name:      "readstore_writer_errors_total",
			Help:      "Total number of read-store writer errors per controller and operation (replace, delete).",
		},
		[]string{subsystemController, "operation"},
	)

	// DNSFQDNsTotal tracks the total number of FQDNs across all DNS resources.
	DNSFQDNsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemDNS,
			Name:      "fqdns_total",
			Help:      "Total number of FQDNs per portal and source.",
		},
		[]string{labelPortal, labelSource},
	)

	// DNSGroupsTotal tracks the number of DNS groups per portal.
	DNSGroupsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemDNS,
			Name:      "groups_total",
			Help:      "Total number of DNS groups per portal.",
		},
		[]string{labelPortal},
	)

	// DNSEntriesValid tracks the number of valid entries projected into auto
	// DNSRecords on the last reconcile, per DNS resource and source kind. Keyed
	// by (namespace, name) — not portal — so multiple DNS CRs sharing a portalRef
	// do not clobber each other's series. Stale series are reclaimed by
	// ValidateEntriesHandler (kinds no longer produced) and ResetDNSEntryMetrics
	// (DNS CR deleted).
	DNSEntriesValid = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemDNS,
			Name:      "entries_valid",
			Help:      "Number of valid entries projected on the last reconcile, per DNS resource (namespace, name) and source kind.",
		},
		[]string{labelNamespace, labelName, labelKind},
	)

	// DNSEntriesInvalid counts entries skipped because they failed DNSRecord
	// validation, per DNS resource, source kind and skip reason.
	DNSEntriesInvalid = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemDNS,
			Name:      "entries_invalid_total",
			Help:      "Total number of entries skipped due to validation failure, per DNS resource (namespace, name), source kind and reason.",
		},
		[]string{labelNamespace, labelName, labelKind, "reason"},
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

	// SourceNotifyDropped counts config-change notifications dropped because the
	// notify channel was full. A non-zero rate suggests reconcile work is not
	// keeping up with DNS CR changes; the periodic tick will still catch up.
	SourceNotifyDropped = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemSource,
			Name:      "notify_dropped_total",
			Help:      "Total number of DNS config-change notifications dropped because the notify channel was full, per portal.",
		},
		[]string{labelPortal},
	)

	// SourceKindActive is 1 when at least one DNS CR currently enables the
	// source kind, 0 otherwise. Drives the SourceReconciler's per-kind List
	// loop and tells operators which kinds the global producer is watching.
	SourceKindActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemSource,
			Name:      "kind_active",
			Help:      "1 when at least one DNS CR enables this source kind, 0 otherwise.",
		},
		[]string{labelKind},
	)

	// SourceDropGuardTriggered counts how many times the producer's
	// anti-collapse guard refused to overwrite a kind's cached endpoints because
	// a fresh collection returned zero while the previous state was non-empty
	// (a likely transient discovery failure). A non-zero rate is an alert signal.
	SourceDropGuardTriggered = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemSource,
			Name:      "drop_guard_triggered_total",
			Help:      "Total times the anti-collapse guard preserved previous endpoints instead of applying an empty collection, per source kind.",
		},
		[]string{labelKind},
	)

	// SourceLastSuccessfulSync is the Unix timestamp (seconds) of the last
	// successful endpoint collection that was applied to the store, per kind.
	SourceLastSuccessfulSync = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemSource,
			Name:      "last_successful_sync_timestamp_seconds",
			Help:      "Unix timestamp of the last successful endpoint collection applied to the store, per source kind.",
		},
		[]string{labelKind},
	)

	// SourceEnrichmentFailures counts endpoints that were kept WITHOUT their
	// source-object metadata (labels/annotations, incl. sreportal.io/groups)
	// because the re-fetch from the cache failed or the external-dns "resource"
	// label was malformed. The endpoint is never dropped (§6), but a non-zero
	// rate means some FQDNs are published without group/origin metadata —
	// alert on it. reason: "fetch" (re-fetch error) or "label" (malformed ref).
	SourceEnrichmentFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemSource,
			Name:      "enrichment_failures_total",
			Help:      "Total endpoints kept without source metadata due to a re-fetch error or malformed resource label, per kind and reason.",
		},
		[]string{labelKind, "reason"},
	)

	// DNSTargetsConflictTotal counts target conflicts observed by the FQDN
	// store when two DNSRecords contribute mismatching targets for the same
	// (name, recordType, portal). First-writer wins; this counter increments
	// once per losing replay.
	DNSTargetsConflictTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemDNS,
			Name:      "targets_conflict_total",
			Help:      "Total number of target conflicts in the FQDN store (loser writes), per portal.",
		},
		[]string{labelPortal},
	)

	// DNSFQDNDedupRatio measures the dedup gain of the FQDN store, per portal:
	// (raw_writes - unique_keys) / raw_writes, where raw_writes is the total
	// number of contributions from DNSRecords assigned to that portal and
	// unique_keys is the number of distinct (name, recordType) entries
	// exposed for it. Range [0, 1); 0 means every contribution is unique.
	DNSFQDNDedupRatio = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemDNS,
			Name:      "fqdn_dedup_ratio",
			Help:      "FQDN store dedup ratio per portal: (raw_writes - unique_keys) / raw_writes.",
		},
		[]string{labelPortal},
	)

	// DNSFQDNRefCount observes the number of contributing DNSRecords backing
	// each FQDN key, sampled on every store write that affects the key.
	// Buckets favour the small-N regime expected in normal operation while
	// still surfacing pathological fan-in.
	DNSFQDNRefCount = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemDNS,
			Name:      "fqdn_refcount",
			Help:      "Distribution of the number of DNSRecord contributors per FQDN key, sampled on each affecting write.",
			Buckets:   []float64{1, 2, 3, 5, 8, 13, 21, 34, 55, 89},
		},
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
		[]string{"method", labelHandler, "code"},
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
		[]string{"method", labelHandler},
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

// --- Image registry metrics ---

var (
	// ImageRegistryEntriesTotal tracks the number of image entries per (portal, host, namespace).
	ImageRegistryEntriesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemImageRegistry,
			Name:      "entries_total",
			Help:      "Total number of image entries per (portal, host, namespace).",
		},
		[]string{labelPortal, labelHost, labelNamespace},
	)

	// ImageRegistryUpgradesTotal tracks the number of entries with UpgradeAvailable=true.
	ImageRegistryUpgradesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemImageRegistry,
			Name:      "upgrades_total",
			Help:      "Number of images with an upgrade available per (portal, host, namespace).",
		},
		[]string{labelPortal, labelHost, labelNamespace},
	)

	// ImageRegistryMutatedTotal tracks the number of entries with ChangeType=mutated.
	ImageRegistryMutatedTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemImageRegistry,
			Name:      "mutated_total",
			Help:      "Number of mutated images per (portal, host, namespace).",
		},
		[]string{labelPortal, labelHost, labelNamespace},
	)

	// ImageRegistryInjectedTotal tracks the number of entries with ChangeType=injected.
	ImageRegistryInjectedTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemImageRegistry,
			Name:      "injected_total",
			Help:      "Number of injected images per (portal, host, namespace).",
		},
		[]string{labelPortal, labelHost, labelNamespace},
	)

	// RegistryLookupTotal counts registry tag-list calls by host and result.
	RegistryLookupTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemImageRegistry,
			Name:      "lookup_total",
			Help:      "Total number of registry tag-list calls by host and result (success, error, rate_limited, skipped, unparsable).",
		},
		[]string{labelHost, labelResult},
	)

	// RegistryLookupDuration tracks the duration of registry tag-list calls.
	RegistryLookupDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemImageRegistry,
			Name:      "lookup_duration_seconds",
			Help:      "Duration of registry tag-list calls in seconds, by host.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{labelHost},
	)
)

// ResetImageRegistryMetrics removes the Gauge label-sets for the given (portal,
// host, namespace) triplet. Called by the ImageRegistry finalizer.
func ResetImageRegistryMetrics(portal, host, namespace string) {
	ImageRegistryEntriesTotal.DeleteLabelValues(portal, host, namespace)
	ImageRegistryUpgradesTotal.DeleteLabelValues(portal, host, namespace)
	ImageRegistryMutatedTotal.DeleteLabelValues(portal, host, namespace)
	ImageRegistryInjectedTotal.DeleteLabelValues(portal, host, namespace)
}

// ResetDNSEntryMetrics removes every entries_valid / entries_invalid_total
// series for the given DNS resource (all kinds/reasons). Called from the DNS
// reconcile when the DNS CR is gone (Get → NotFound) so a deleted resource does
// not leave phantom series behind.
func ResetDNSEntryMetrics(namespace, name string) {
	DNSEntriesValid.DeletePartialMatch(prometheus.Labels{labelNamespace: namespace, labelName: name})
	DNSEntriesInvalid.DeletePartialMatch(prometheus.Labels{labelNamespace: namespace, labelName: name})
}

// DeleteDNSEntriesValidSeries removes the entries_valid gauge series (all kinds)
// for the given DNS resource. Called each reconcile before re-setting so a kind
// that stopped producing does not leave a frozen non-zero gauge. Only the gauge
// is dropped — the invalid counter is cumulative and must survive.
func DeleteDNSEntriesValidSeries(namespace, name string) {
	DNSEntriesValid.DeletePartialMatch(prometheus.Labels{labelNamespace: namespace, labelName: name})
}

func init() {
	// Register all metrics with the controller-runtime metrics registry
	// so they are exposed on the /metrics endpoint.
	metrics.Registry.MustRegister(
		// Controller
		ReconcileTotal,
		ReconcileDuration,
		ReadstoreWriterErrors,
		// DNS
		DNSFQDNsTotal,
		DNSGroupsTotal,
		DNSEntriesValid,
		DNSEntriesInvalid,
		// Source
		SourceEndpointsCollected,
		SourceErrorsTotal,
		SourceSkippedUpdates,
		SourceNotifyDropped,
		SourceKindActive,
		SourceDropGuardTriggered,
		SourceLastSuccessfulSync,
		SourceEnrichmentFailures,
		// DNS conflicts
		DNSTargetsConflictTotal,
		// DNS readstore
		DNSFQDNDedupRatio,
		DNSFQDNRefCount,
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
		// Image registry
		ImageRegistryEntriesTotal,
		ImageRegistryUpgradesTotal,
		ImageRegistryMutatedTotal,
		ImageRegistryInjectedTotal,
		RegistryLookupTotal,
		RegistryLookupDuration,
	)
}
