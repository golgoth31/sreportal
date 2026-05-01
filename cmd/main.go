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

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	externaldnsv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"

	"sigs.k8s.io/external-dns/source/annotations"

	sreportal "github.com/golgoth31/sreportal"
	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/alertmanagerclient"
	"github.com/golgoth31/sreportal/internal/auth"
	"github.com/golgoth31/sreportal/internal/config"
	alertmanagerctrl "github.com/golgoth31/sreportal/internal/controller/alertmanager"
	componentctrl "github.com/golgoth31/sreportal/internal/controller/component"
	dnsctrl "github.com/golgoth31/sreportal/internal/controller/dns"
	dnschain "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	dnsrecordsctrl "github.com/golgoth31/sreportal/internal/controller/dnsrecords"
	emojictrl "github.com/golgoth31/sreportal/internal/controller/emoji"
	imagectrl "github.com/golgoth31/sreportal/internal/controller/image"
	imageinventoryctrl "github.com/golgoth31/sreportal/internal/controller/imageinventory"
	incidentctrl "github.com/golgoth31/sreportal/internal/controller/incident"
	maintenancectrl "github.com/golgoth31/sreportal/internal/controller/maintenance"
	nfdctrl "github.com/golgoth31/sreportal/internal/controller/networkflowdiscovery"
	nfdchain "github.com/golgoth31/sreportal/internal/controller/networkflowdiscovery/chain"
	portalctrl "github.com/golgoth31/sreportal/internal/controller/portal"
	portalchain "github.com/golgoth31/sreportal/internal/controller/portal/chain"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	releasectrl "github.com/golgoth31/sreportal/internal/controller/release"
	sourcectrl "github.com/golgoth31/sreportal/internal/controller/source"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/mcp"
	alertmanagerreadstore "github.com/golgoth31/sreportal/internal/readstore/alertmanager"
	componentreadstore "github.com/golgoth31/sreportal/internal/readstore/component"
	dnsreadstore "github.com/golgoth31/sreportal/internal/readstore/dns"
	emojireadstore "github.com/golgoth31/sreportal/internal/readstore/emoji"
	imagereadstore "github.com/golgoth31/sreportal/internal/readstore/image"
	incidentreadstore "github.com/golgoth31/sreportal/internal/readstore/incident"
	maintenancereadstore "github.com/golgoth31/sreportal/internal/readstore/maintenance"
	netpolreadstore "github.com/golgoth31/sreportal/internal/readstore/netpol"
	portalreadstore "github.com/golgoth31/sreportal/internal/readstore/portal"
	releasereadstore "github.com/golgoth31/sreportal/internal/readstore/release"
	releaseservice "github.com/golgoth31/sreportal/internal/release"
	"github.com/golgoth31/sreportal/internal/remoteclient"
	"github.com/golgoth31/sreportal/internal/slackclient"
	"github.com/golgoth31/sreportal/internal/source"
	statuspagesvc "github.com/golgoth31/sreportal/internal/statuspage"
	"github.com/golgoth31/sreportal/internal/version"
	webhookv1alpha1 "github.com/golgoth31/sreportal/internal/webhook/v1alpha1"
	"github.com/golgoth31/sreportal/internal/webserver"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// Register external-dns DNSEndpoint types with full scheme support
	utilruntime.Must(externaldnsv1alpha1.AddToScheme(scheme))

	utilruntime.Must(sreportalv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var webAddr string
	var webRoot string
	var devMode bool
	var secureMetrics bool
	var enableHTTP2 bool
	var configPath string
	var portalNamespace string
	var enableMCP bool
	var mcpTransport string
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&webAddr, "web-bind-address", ":8090", "The address the web UI server binds to.")
	flag.StringVar(&webRoot, "web-root", "web/dist/web/browser",
		"The path to the Angular dist directory (only used in dev mode).")
	flag.BoolVar(&devMode, "dev-mode", false,
		"Enable dev mode: serve the web UI from the filesystem (--web-root) instead of the embedded binary.")
	flag.StringVar(&configPath, "config", config.DefaultConfigPath,
		"Path to the operator configuration file.")
	flag.StringVar(&portalNamespace, "portal-namespace", "sreportal-system",
		"The namespace where the main portal will be auto-created.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":9090", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	var webhookPort int
	flag.IntVar(&webhookPort, "webhook-port", 9443, "The port the webhook server binds to.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.BoolVar(&enableMCP, "enable-mcp", false,
		"If set, the MCP (Model Context Protocol) server will be enabled for AI assistant integration.")
	flag.StringVar(&mcpTransport, "mcp-transport", "streamable-http",
		"The transport to use for the MCP server: 'stdio' or 'streamable-http'.")
	var corsAllowedOrigins string
	flag.StringVar(&corsAllowedOrigins, "cors-allowed-origins", "",
		"Comma-separated list of origins allowed for CORS requests (e.g. http://localhost:5173). "+
			"Leave empty to disable CORS. In dev mode, http://localhost:5173 is added automatically.")
	var logCfg log.Config
	logCfg.BindFlags(flag.CommandLine)
	flag.Parse()

	if devMode && logCfg.Level == log.LevelInfoValue {
		logCfg.Level = log.LevelDebugValue
	}
	logCfg.AddCaller = true
	logCfg.DevMode = devMode
	if err := log.Init(logCfg); err != nil {
		// Cannot use setupLog yet — fall back to stderr.
		fmt.Fprintf(os.Stderr, "failed to initialise logger: %v\n", err)
		os.Exit(1)
	}
	ctrl.SetLogger(log.Default().ToLogr())

	setupLog := log.Default().WithName("setup")

	// Environment variables (Kubernetes Downward API)
	podName := os.Getenv("POD_NAME")
	podNamespace := os.Getenv("POD_NAMESPACE")

	if podNamespace != "" {
		portalNamespace = podNamespace
	}

	setupLog.Info("sreportal", "version", version.Version, "commit", version.Commit, "date", version.Date,
		"podName", podName, "podNamespace", podNamespace, "portalNamespace", portalNamespace)

	// Load operator configuration from file
	operatorConfig, err := config.LoadFromFile(configPath)
	if err != nil {
		setupLog.Error(err, "failed to load configuration", "path", configPath)
		os.Exit(1)
	}
	setupLog.Info("loaded configuration", "path", configPath, "config", operatorConfig.LogSummary())

	// Build authentication chain from operator configuration.
	// API key secret is read from an environment variable (populated by a K8s Secret).
	var authChain *auth.Chain
	if operatorConfig.Auth.Enabled() {
		var authenticators []auth.Authenticator
		if operatorConfig.Auth.APIKey != nil && operatorConfig.Auth.APIKey.Enabled {
			const headerAPIKeyEnv = "HEADER_API_KEY"
			apiKey := os.Getenv(headerAPIKeyEnv)
			if apiKey == "" {
				setupLog.Error(nil, "auth: HEADER_API_KEY env var is empty")
				os.Exit(1)
			}
			authenticators = append(authenticators,
				auth.NewAPIKeyAuthenticator(operatorConfig.Auth.APIKey.HeaderName, apiKey))
			setupLog.Info("auth: API key auth enabled",
				"header", operatorConfig.Auth.APIKey.HeaderName)
		}
		if operatorConfig.Auth.JWT != nil && operatorConfig.Auth.JWT.Enabled {
			jwtAuth, err := auth.NewJWTAuthenticator(context.Background(), *operatorConfig.Auth.JWT)
			if err != nil {
				setupLog.Error(err, "failed to initialize JWT authenticator")
				os.Exit(1)
			}
			defer jwtAuth.Close()
			authenticators = append(authenticators, jwtAuth)
			setupLog.Info("auth: JWT auth enabled", "issuers", len(operatorConfig.Auth.JWT.Issuers))
		}
		authChain = auth.NewChain(authenticators...)
		setupLog.Info("auth: write endpoints protected", "methods", len(authenticators))
	} else {
		setupLog.Warn("auth: authentication is DISABLED — write endpoints are unprotected")
	}

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
		Port:    webhookPort,
	}

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "198706f3.my.domain",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Add field indexer for DNSRecord.spec.portalRef
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.DNSRecord{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			dnsRecord := o.(*sreportalv1alpha1.DNSRecord)
			if dnsRecord.Spec.PortalRef == "" {
				return nil
			}
			return []string{dnsRecord.Spec.PortalRef}
		},
	); err != nil {
		setupLog.Error(err, "unable to create field indexer", "field", portalfeatures.FieldIndexPortalRef)
		os.Exit(1)
	}

	// Add field indexer for NetworkFlowDiscovery.spec.portalRef
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.NetworkFlowDiscovery{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			nfd := o.(*sreportalv1alpha1.NetworkFlowDiscovery)
			if nfd.Spec.PortalRef == "" {
				return nil
			}
			return []string{nfd.Spec.PortalRef}
		},
	); err != nil {
		setupLog.Error(err, "unable to create field indexer",
			"field", portalfeatures.FieldIndexPortalRef, "kind", "NetworkFlowDiscovery")
		os.Exit(1)
	}

	// Add field indexer for DNS.spec.portalRef
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.DNS{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			dns := o.(*sreportalv1alpha1.DNS)
			if dns.Spec.PortalRef == "" {
				return nil
			}
			return []string{dns.Spec.PortalRef}
		},
	); err != nil {
		setupLog.Error(err, "unable to create field indexer", "field", portalfeatures.FieldIndexPortalRef)
		os.Exit(1)
	}

	// Add field indexer for Release.spec.portalRef
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.Release{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			rel := o.(*sreportalv1alpha1.Release)
			if rel.Spec.PortalRef == "" {
				return nil
			}
			return []string{rel.Spec.PortalRef}
		},
	); err != nil {
		setupLog.Error(err, "unable to create field indexer",
			"field", portalfeatures.FieldIndexPortalRef, "kind", "Release")
		os.Exit(1)
	}

	// Add field indexer for Alertmanager.spec.portalRef
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.Alertmanager{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			am := o.(*sreportalv1alpha1.Alertmanager)
			if am.Spec.PortalRef == "" {
				return nil
			}
			return []string{am.Spec.PortalRef}
		},
	); err != nil {
		setupLog.Error(err, "unable to create field indexer",
			"field", portalfeatures.FieldIndexPortalRef, "kind", "Alertmanager")
		os.Exit(1)
	}

	// Add field indexer for Component.spec.portalRef
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.Component{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			comp := o.(*sreportalv1alpha1.Component)
			if comp.Spec.PortalRef == "" {
				return nil
			}
			return []string{comp.Spec.PortalRef}
		},
	); err != nil {
		setupLog.Error(err, "unable to create field indexer",
			"field", portalfeatures.FieldIndexPortalRef, "kind", "Component")
		os.Exit(1)
	}

	// Add field indexer for Incident.spec.portalRef
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.Incident{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			inc := o.(*sreportalv1alpha1.Incident)
			if inc.Spec.PortalRef == "" {
				return nil
			}
			return []string{inc.Spec.PortalRef}
		},
	); err != nil {
		setupLog.Error(err, "unable to create field indexer",
			"field", portalfeatures.FieldIndexPortalRef, "kind", "Incident")
		os.Exit(1)
	}

	// Add field indexer for Maintenance.spec.portalRef
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.Maintenance{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			maint := o.(*sreportalv1alpha1.Maintenance)
			if maint.Spec.PortalRef == "" {
				return nil
			}
			return []string{maint.Spec.PortalRef}
		},
	); err != nil {
		setupLog.Error(err, "unable to create field indexer",
			"field", portalfeatures.FieldIndexPortalRef, "kind", "Maintenance")
		os.Exit(1)
	}

	// Add field indexer for ImageInventory.spec.portalRef
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.ImageInventory{},
		portalfeatures.FieldIndexPortalRef,
		func(o client.Object) []string {
			inv := o.(*sreportalv1alpha1.ImageInventory)
			if inv.Spec.PortalRef == "" {
				return nil
			}
			return []string{inv.Spec.PortalRef}
		},
	); err != nil {
		setupLog.Error(err, "unable to create field indexer",
			"field", portalfeatures.FieldIndexPortalRef, "kind", "ImageInventory")
		os.Exit(1)
	}

	// Create kubernetes clientset for external-dns sources
	restConfig := ctrl.GetConfigOrDie()
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		setupLog.Error(err, "unable to create kubernetes client")
		os.Exit(1)
	}

	annotations.SetAnnotationPrefix("external-dns.alpha.kubernetes.io/")

	// Create ReadStores: controllers write, gRPC/MCP read.
	sourceBuilders := source.DefaultBuilders()
	var sourcePriority []string
	var disableDNSCheck bool
	var groupMapping *config.GroupMappingConfig
	if operatorConfig != nil {
		sourcePriority = source.FilterPriorityOrder(operatorConfig.Sources.Priority, sourceBuilders, operatorConfig)
		disableDNSCheck = operatorConfig.Reconciliation.DisableDNSCheck
		groupMapping = &operatorConfig.GroupMapping
	}
	fqdnStore := dnsreadstore.NewFQDNStore(nil) // priority dedup handled in SourceReconciler
	portalStore := portalreadstore.NewPortalStore()
	releaseStore := releasereadstore.NewReleaseStore()
	alertmanagerStore := alertmanagerreadstore.NewAlertmanagerStore()
	imageStore := imagereadstore.NewStore()
	emojiStore := emojireadstore.NewEmojiStore()

	// Emoji: Slack custom emoji sync (optional, async at startup + periodic refresh)
	slackEnabled := operatorConfig != nil && operatorConfig.Emoji != nil &&
		operatorConfig.Emoji.Slack != nil && operatorConfig.Emoji.Slack.Enabled
	if slackEnabled {
		slackToken := os.Getenv("SLACK_API_TOKEN")
		if slackToken != "" {
			interval := operatorConfig.Emoji.Slack.RefreshInterval.Duration()
			if interval == 0 {
				interval = 24 * time.Hour
			}
			slackClient := slackclient.NewClient(slackToken)
			emojiRunnable := emojictrl.NewEmojiRunnable(slackClient, emojiStore, interval)
			if err := mgr.Add(emojiRunnable); err != nil {
				setupLog.Error(err, "unable to add emoji runnable")
				os.Exit(1)
			}
			setupLog.Info("emoji: Slack custom emoji sync enabled", "interval", interval)
		} else {
			setupLog.Info("emoji: Slack custom emoji sync disabled (SLACK_API_TOKEN not set)")
		}
	} else {
		setupLog.Info("emoji: Slack custom emoji sync disabled (not configured)")
	}

	dnsReconciler := dnsctrl.NewDNSReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		disableDNSCheck,
	)
	dnsReconciler.SetFQDNWriter(fqdnStore)
	if err := dnsReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DNS")
		os.Exit(1)
	}

	dnsRecordReconciler := dnsrecordsctrl.NewDNSRecordReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		groupMapping,
		dnschain.NewNetResolver(),
		disableDNSCheck,
	)
	dnsRecordReconciler.SetFQDNWriter(fqdnStore)
	if err := dnsRecordReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DNSRecord")
		os.Exit(1)
	}

	sourceReconciler := sourcectrl.NewSourceReconciler(
		mgr.GetClient(),
		kubeClient,
		restConfig,
		operatorConfig,
		sourceBuilders,
		sourcePriority,
	)
	if err := sourceReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Source")
		os.Exit(1)
	}

	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1alpha1.SetupDNSWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "DNS")
			os.Exit(1)
		}
		if err := webhookv1alpha1.SetupPortalWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Portal")
			os.Exit(1)
		}
		if err := webhookv1alpha1.SetupReleaseWebhookWithManager(mgr, operatorConfig.Release.Types); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Release")
			os.Exit(1)
		}
		if err := webhookv1alpha1.SetupImageInventoryWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "ImageInventory")
			os.Exit(1)
		}
	}
	remoteCache := remoteclient.NewCache()
	portalReconciler := portalctrl.NewPortalReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		remoteCache,
	)
	portalReconciler.SetPortalWriter(portalStore)
	portalReconciler.SetFQDNWriter(fqdnStore)
	portalReconciler.SetReleaseWriter(releaseStore)
	if err := portalReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Portal")
		os.Exit(1)
	}

	// Add runnable to ensure main portal exists at startup
	if err := mgr.Add(portalchain.NewEnsureMainPortalRunnable(
		mgr.GetClient(),
		mgr.GetCache(),
		portalNamespace,
	)); err != nil {
		setupLog.Error(err, "unable to add main portal ensure runnable")
		os.Exit(1)
	}

	// Add runnable to ensure NetworkFlowDiscovery exists for the main portal
	if err := mgr.Add(nfdchain.NewEnsureNFDRunnable(
		mgr.GetClient(),
		mgr.GetCache(),
		portalNamespace,
		portalchain.MainPortalName,
	)); err != nil {
		setupLog.Error(err, "unable to add NFD ensure runnable")
		os.Exit(1)
	}
	amClient := alertmanagerclient.NewClient()
	amReconciler := alertmanagerctrl.NewAlertmanagerReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		amClient,
		amClient,
		remoteCache,
	)
	amReconciler.SetAlertmanagerWriter(alertmanagerStore)
	if err := amReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Alertmanager")
		os.Exit(1)
	}
	flowGraphStore := netpolreadstore.NewFlowGraphStore()
	portalReconciler.SetFlowGraphWriter(flowGraphStore)
	nfdReconciler := nfdctrl.NewNetworkFlowDiscoveryReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		remoteCache,
	)
	nfdReconciler.SetFlowGraphWriter(flowGraphStore)
	if err := nfdReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NetworkFlowDiscovery")
		os.Exit(1)
	}
	// Status page stores and controllers
	componentStore := componentreadstore.NewComponentStore()
	maintenanceStore := maintenancereadstore.NewMaintenanceStore()
	incidentStore := incidentreadstore.NewIncidentStore()

	componentReconciler := componentctrl.NewComponentReconciler(
		mgr.GetClient(), maintenanceStore, componentStore,
	)
	if err := componentReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Component")
		os.Exit(1)
	}

	maintenanceReconciler := maintenancectrl.NewMaintenanceReconciler(mgr.GetClient(), maintenanceStore)
	if err := maintenanceReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Maintenance")
		os.Exit(1)
	}

	incidentReconciler := incidentctrl.NewIncidentReconciler(mgr.GetClient(), incidentStore)
	if err := incidentReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Incident")
		os.Exit(1)
	}
	imageWorkloadHandler := imagectrl.NewWorkloadHandler(mgr.GetClient(), imageStore)
	if err := imagectrl.SetupWorkloadReconcilersWithManager(mgr, imageWorkloadHandler); err != nil {
		setupLog.Error(err, "unable to set up workload image reconcilers")
		os.Exit(1)
	}
	imageInventoryReconciler := imageinventoryctrl.NewImageInventoryReconciler(mgr.GetClient(), imageStore, remoteCache)
	if err := imageInventoryReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "ImageInventory")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Derive a cancellable context from the signal handler so that fatal
	// errors in background servers (web, MCP) can trigger a clean shutdown.
	signalCtx := ctrl.SetupSignalHandler()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()

	// Create Release service
	releaseNamespace := operatorConfig.Release.Namespace
	if releaseNamespace == "" {
		releaseNamespace = portalNamespace
	}
	releaseTTL := operatorConfig.Release.TTL.Duration()
	if releaseTTL == 0 {
		releaseTTL = 30 * 24 * 60 * 60 * 1e9 // 30 days in nanoseconds
	}
	releaseSvc := releaseservice.NewService(mgr.GetClient(), releaseNamespace, portalchain.MainPortalName)

	// Release controller: watches Release CRs, pushes to ReadStore, and deletes expired CRs
	releaseReconciler := releasectrl.NewReleaseReconciler(mgr.GetClient(), releaseTTL)
	releaseReconciler.SetReleaseWriter(releaseStore)
	if err := releaseReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Release")
		os.Exit(1)
	}

	// Start the web server in a goroutine
	webCfg := webserver.Config{
		Address:             webAddr,
		Gatherer:            ctrlmetrics.Registry,
		ReleaseReader:       releaseStore,
		ReleaseService:      releaseSvc,
		ReleaseTTL:          releaseTTL,
		ReleaseAllowedTypes: operatorConfig.Release.Types,
		FQDNReader:          fqdnStore,
		PortalReader:        portalStore,
		AlertmanagerReader:  alertmanagerStore,
		FlowGraphReader:     flowGraphStore,
		ComponentReader:     componentStore,
		MaintenanceReader:   maintenanceStore,
		IncidentReader:      incidentStore,
		ImageReader:         imageStore,
		StatusPageService:   statuspagesvc.NewService(mgr.GetClient(), portalNamespace),
		EmojiReader:         emojiStore,
		AuthChain:           authChain,
	}
	if devMode {
		setupLog.Info("dev mode enabled: serving web UI from filesystem", "web-root", webRoot)
		webCfg.WebRoot = webRoot
	} else {
		// Serve the web UI from the embedded filesystem.
		// The embed.FS contains files under "web/dist/web/browser/...",
		// so we strip that prefix to serve from the root.
		subFS, err := fs.Sub(sreportal.WebUI, "web/dist/web/browser")
		if err != nil {
			setupLog.Error(err, "unable to create sub filesystem for embedded web UI")
			os.Exit(1)
		}
		setupLog.Info("serving web UI from embedded filesystem")
		webCfg.WebFS = subFS
	}
	// Build allowed origins: explicit flag + localhost:5173 in dev mode.
	var corsOrigins []string
	for o := range strings.SplitSeq(corsAllowedOrigins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			corsOrigins = append(corsOrigins, o)
		}
	}
	if devMode {
		corsOrigins = append(corsOrigins, "http://localhost:5173")
	}
	webServer := webserver.New(webCfg, mgr.GetClient(), operatorConfig, corsOrigins)

	// Start MCP servers if enabled
	if enableMCP {
		dnsMcpServer := mcp.NewDNSServer(fqdnStore, portalStore)
		alertsMcpServer := mcp.NewAlertsServer(alertmanagerStore)
		metricsMcpServer := mcp.NewMetricsServer(ctrlmetrics.Registry)
		releasesMcpServer := mcp.NewReleasesServer(releaseStore)
		netpolMcpServer := mcp.NewNetpolServer(flowGraphStore)
		statusMcpServer := mcp.NewStatusServer(componentStore, maintenanceStore, incidentStore)
		imageMcpServer := mcp.NewImageServer(imageStore)

		switch mcpTransport {
		case "stdio":
			go func() {
				setupLog.Info("starting MCP DNS server", "transport", "stdio")
				if err := dnsMcpServer.ServeStdio(); err != nil {
					setupLog.Error(err, "MCP DNS server failed, initiating shutdown")
					cancel()
				}
			}()
		case "streamable-http":
			setupLog.Info("mounting MCP servers on web server",
				"dns", []string{"/mcp", "/mcp/dns"},
				"alerts", "/mcp/alerts",
				"metrics", "/mcp/metrics",
				"releases", "/mcp/releases",
				"netpol", "/mcp/netpol",
				"status", "/mcp/status",
				"image", "/mcp/image",
			)
			webServer.MountHandler("/mcp", dnsMcpServer.Handler())
			webServer.MountHandler("/mcp/dns", dnsMcpServer.Handler())
			webServer.MountHandler("/mcp/alerts", alertsMcpServer.Handler())
			webServer.MountHandler("/mcp/metrics", metricsMcpServer.Handler())
			webServer.MountHandler("/mcp/releases", releasesMcpServer.Handler())
			webServer.MountHandler("/mcp/netpol", netpolMcpServer.Handler())
			webServer.MountHandler("/mcp/status", statusMcpServer.Handler())
			webServer.MountHandler("/mcp/image", imageMcpServer.Handler())
		default:
			setupLog.Error(nil, "unknown MCP transport", "transport", mcpTransport)
			os.Exit(1)
		}
	}

	go func() {
		setupLog.Info("starting web server", "address", webAddr)
		if err := webServer.Start(); err != nil {
			setupLog.Error(err, "web server failed, initiating shutdown")
			cancel()
		}
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	// Gracefully shutdown web server
	if err := webServer.Shutdown(context.Background()); err != nil {
		setupLog.Error(err, "error shutting down web server")
	}
}
