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
	"io/fs"
	"os"

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
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	externaldnsv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"

	sreportal "github.com/golgoth31/sreportal"
	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/controller"
	portalctrl "github.com/golgoth31/sreportal/internal/controller/portal"
	"github.com/golgoth31/sreportal/internal/mcp"
	"github.com/golgoth31/sreportal/internal/version"
	webhookv1alpha1 "github.com/golgoth31/sreportal/internal/webhook/v1alpha1"
	"github.com/golgoth31/sreportal/internal/webserver"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
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
	var mcpAddr string
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
	flag.StringVar(&mcpTransport, "mcp-transport", "stdio",
		"The transport to use for the MCP server: 'stdio' or 'streamable-http'.")
	flag.StringVar(&mcpAddr, "mcp-bind-address", ":8091",
		"The address the MCP server binds to (only used when mcp-transport is 'streamable-http').")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

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
		"spec.portalRef",
		func(o client.Object) []string {
			dnsRecord := o.(*sreportalv1alpha1.DNSRecord)
			if dnsRecord.Spec.PortalRef == "" {
				return nil
			}
			return []string{dnsRecord.Spec.PortalRef}
		},
	); err != nil {
		setupLog.Error(err, "unable to create field indexer", "field", "spec.portalRef")
		os.Exit(1)
	}

	// Add field indexer for DNS.spec.portalRef
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.DNS{},
		"spec.portalRef",
		func(o client.Object) []string {
			dns := o.(*sreportalv1alpha1.DNS)
			if dns.Spec.PortalRef == "" {
				return nil
			}
			return []string{dns.Spec.PortalRef}
		},
	); err != nil {
		setupLog.Error(err, "unable to create field indexer", "field", "spec.portalRef")
		os.Exit(1)
	}

	if err := controller.NewDNSReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		operatorConfig,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DNS")
		os.Exit(1)
	}

	// Create kubernetes clientset for external-dns sources
	restConfig := ctrl.GetConfigOrDie()
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		setupLog.Error(err, "unable to create kubernetes client")
		os.Exit(1)
	}

	// Source reconciler for external-dns source integration
	sourceReconciler := controller.NewSourceReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		kubeClient,
		restConfig,
		operatorConfig,
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
	}
	if err := controller.NewPortalReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Portal")
		os.Exit(1)
	}

	// Add runnable to ensure main portal exists at startup
	if err := mgr.Add(portalctrl.NewEnsureMainPortalRunnable(
		mgr.GetClient(),
		mgr.GetCache(),
		portalNamespace,
	)); err != nil {
		setupLog.Error(err, "unable to add main portal ensure runnable")
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

	// Start the web server in a goroutine
	webCfg := webserver.Config{
		Address: webAddr,
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
	webServer := webserver.New(webCfg, mgr.GetClient(), operatorConfig)

	go func() {
		setupLog.Info("starting web server", "address", webAddr)
		if err := webServer.Start(); err != nil {
			setupLog.Error(err, "web server failed, initiating shutdown")
			cancel()
		}
	}()

	// Start MCP server if enabled
	var mcpServer *mcp.Server
	if enableMCP {
		mcpServer = mcp.New(mgr.GetClient(), &operatorConfig.GroupMapping)
		switch mcpTransport {
		case "stdio":
			go func() {
				setupLog.Info("starting MCP server", "transport", "stdio")
				if err := mcpServer.ServeStdio(); err != nil {
					setupLog.Error(err, "MCP server failed, initiating shutdown")
					cancel()
				}
			}()
		case "streamable-http":
			go func() {
				setupLog.Info("starting MCP server", "transport", "streamable-http", "address", mcpAddr)
				if err := mcpServer.ServeStreamableHTTP(mcpAddr); err != nil {
					setupLog.Error(err, "MCP server failed, initiating shutdown")
					cancel()
				}
			}()
		default:
			setupLog.Error(nil, "unknown MCP transport", "transport", mcpTransport)
			os.Exit(1)
		}
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	// Gracefully shutdown web server
	if err := webServer.Shutdown(context.Background()); err != nil {
		setupLog.Error(err, "error shutting down web server")
	}

	// Gracefully shutdown MCP server
	if mcpServer != nil {
		if err := mcpServer.Shutdown(context.Background()); err != nil {
			setupLog.Error(err, "error shutting down MCP server")
		}
	}
}
