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

package webserver

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golgoth31/sreportal/internal/auth"
	"github.com/golgoth31/sreportal/internal/log"

	"github.com/golgoth31/sreportal/internal/config"
	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanagerreadmodel"
	domaincomponent "github.com/golgoth31/sreportal/internal/domain/component"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	domainemoji "github.com/golgoth31/sreportal/internal/domain/emoji"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	domainincident "github.com/golgoth31/sreportal/internal/domain/incident"
	domainmaint "github.com/golgoth31/sreportal/internal/domain/maintenance"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	"github.com/golgoth31/sreportal/internal/grpc"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/openapi"
	releaseservice "github.com/golgoth31/sreportal/internal/release"
	statuspagesvc "github.com/golgoth31/sreportal/internal/statuspage"
)

// Config holds the web server configuration
type Config struct {
	// Address is the address to listen on (e.g., ":8080")
	Address string

	// WebRoot is the path to the Angular dist directory
	WebRoot string

	// WebFS is an optional embedded filesystem for the web UI
	WebFS fs.FS

	// Gatherer is the Prometheus metrics gatherer for the MetricsService
	Gatherer prometheus.Gatherer

	// ReleaseReader is the read-side interface for Release data (provided by the ReadStore)
	ReleaseReader domainrelease.ReleaseReader

	// ReleaseService is the shared release service for the write path (AddEntry)
	ReleaseService *releaseservice.Service

	// ReleaseTTL is the TTL for release CRs, exposed to the frontend via ListReleaseDays
	ReleaseTTL time.Duration

	// ReleaseAllowedTypes is the list of allowed release types with display config (empty means all allowed)
	ReleaseAllowedTypes []config.ReleaseTypeConfig

	// FQDNReader is the read-side interface for DNS data (provided by the ReadStore)
	FQDNReader domaindns.FQDNReader

	// PortalReader is the read-side interface for Portal data (provided by the ReadStore)
	PortalReader domainportal.PortalReader

	// AlertmanagerReader is the read-side interface for Alertmanager data (provided by the ReadStore)
	AlertmanagerReader domainalertmanager.AlertmanagerReader

	// FlowGraphReader is the read-side interface for network flow graph data (provided by the ReadStore)
	FlowGraphReader domainnetpol.FlowGraphReader

	// ComponentReader is the read-side interface for Component data (provided by the ReadStore)
	ComponentReader domaincomponent.ComponentReader

	// MaintenanceReader is the read-side interface for Maintenance data (provided by the ReadStore)
	MaintenanceReader domainmaint.MaintenanceReader

	// IncidentReader is the read-side interface for Incident data (provided by the ReadStore)
	IncidentReader domainincident.IncidentReader

	// ImageReader is the read-side interface for Image inventory data.
	ImageReader domainimage.ImageReader

	// StatusPageService is the write-path service for status page CRs (nil = read-only)
	StatusPageService *statuspagesvc.Service

	// EmojiReader is the read-side interface for custom emoji data (provided by the ReadStore)
	EmojiReader domainemoji.EmojiReader

	// AuthChain is the authentication chain for write endpoints (nil = no auth)
	AuthChain *auth.Chain
}

// Server is the web server for the SRE Portal
type Server struct {
	config         Config
	echo           *echo.Echo
	client         client.Client
	operatorConfig *config.OperatorConfig
	httpServer     *http.Server
}

// New creates a new web server.
// allowedOrigins lists the origins permitted for CORS requests.
// Pass an empty slice to disable cross-origin access (production default).
func New(cfg Config, c client.Client, operatorConfig *config.OperatorConfig, allowedOrigins []string) *Server {
	e := echo.New()

	// Request logging middleware — wraps the response writer to capture the
	// real HTTP status code (Connect writes 4xx/5xx directly, bypassing Echo).
	e.Use(requestLoggerMiddleware)
	e.Use(middleware.Recover())
	e.Use(metricsMiddleware)
	if len(allowedOrigins) > 0 {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: allowedOrigins,
		}))
	}

	s := &Server{
		config:         cfg,
		echo:           e,
		client:         c,
		operatorConfig: operatorConfig,
	}

	s.setupRoutes()
	return s
}

// echoConnectMux adapts Echo to connecthttp.ServeMux so Connect can register
// procedures while keeping Echo as the outer router (SPA, swagger, MCP).
type echoConnectMux struct {
	echo *echo.Echo
}

func (m *echoConnectMux) Handle(pattern string, handler http.Handler) {
	if strings.HasSuffix(pattern, "/") {
		// Service catch-all from connecthttp.Mount needs Echo's wildcard.
		m.echo.Any(pattern+"*", echo.WrapHandler(handler))
		return
	}
	m.echo.Any(pattern, echo.WrapHandler(handler))
}

// setupRoutes configures all routes
func (s *Server) setupRoutes() {
	mux := &echoConnectMux{echo: s.echo}

	// Read-only / public Connect services share one server with logging only.
	// Connect returns HTTP 200 even on coded errors, so the interceptor logs
	// them at WARN — they are invisible to the Echo request logger middleware.
	public := connect.NewServer(grpc.LoggingInterceptor())
	sreportalv1connect.RegisterDNSServiceHandler(public, grpc.NewDNSService(s.config.FQDNReader, s.config.PortalReader))
	sreportalv1connect.RegisterPortalServiceHandler(public, grpc.NewPortalService(s.config.PortalReader))
	sreportalv1connect.RegisterAlertmanagerServiceHandler(public, grpc.NewAlertmanagerService(s.config.AlertmanagerReader, s.config.PortalReader))
	sreportalv1connect.RegisterNetworkPolicyServiceHandler(public, grpc.NewNetworkPolicyService(s.config.FlowGraphReader, s.config.PortalReader))
	sreportalv1connect.RegisterVersionServiceHandler(public, grpc.NewVersionService())
	sreportalv1connect.RegisterEmojiServiceHandler(public, grpc.NewEmojiService(s.config.EmojiReader))
	if s.config.ImageReader != nil {
		sreportalv1connect.RegisterImageServiceHandler(public, grpc.NewImageService(s.config.ImageReader, s.config.PortalReader))
	}
	if s.config.Gatherer != nil {
		sreportalv1connect.RegisterMetricsServiceHandler(public, grpc.NewMetricsService(s.config.Gatherer))
	}
	connecthttp.Mount(mux, public)

	// Write-capable services get their own servers so auth can be layered on.
	if s.config.ReleaseReader != nil {
		releaseServer := connect.NewServer(s.writeConnectInterceptors()...)
		sreportalv1connect.RegisterReleaseServiceHandler(releaseServer, grpc.NewReleaseService(
			s.config.ReleaseReader, s.config.ReleaseService, s.config.ReleaseTTL, s.config.ReleaseAllowedTypes, s.config.PortalReader,
		))
		connecthttp.Mount(mux, releaseServer)
	}

	if s.config.ComponentReader != nil {
		statusServer := connect.NewServer(s.writeConnectInterceptors()...)
		sreportalv1connect.RegisterStatusServiceHandler(statusServer, grpc.NewStatusService(
			s.config.ComponentReader,
			s.config.MaintenanceReader,
			s.config.IncidentReader,
			s.config.StatusPageService,
			s.config.PortalReader,
		))
		connecthttp.Mount(mux, statusServer)
	}

	// Swagger UI — serve embedded OpenAPI files at /swagger
	swaggerFS, _ := fs.Sub(openapi.Swagger, "swagger")
	swaggerHandler := http.StripPrefix("/swagger", http.FileServer(http.FS(swaggerFS)))
	s.echo.GET("/swagger/*", echo.WrapHandler(swaggerHandler))
	s.echo.GET("/swagger", func(c *echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})

	// API health check
	s.echo.GET("/api/health", s.healthHandler)

	// Serve static files for Angular SPA
	s.setupStaticFiles()
}

// writeConnectInterceptors returns logging plus optional auth for write-capable Connect servers.
func (s *Server) writeConnectInterceptors() []connect.ServerInterceptor {
	interceptors := []connect.ServerInterceptor{grpc.LoggingInterceptor()}
	if s.config.AuthChain != nil {
		interceptors = append(interceptors, auth.AuthInterceptor(s.config.AuthChain))
	}
	return interceptors
}

// setupStaticFiles configures static file serving for the Angular app
func (s *Server) setupStaticFiles() {
	if s.config.WebFS != nil {
		// Use embedded filesystem
		s.echo.GET("/*", s.spaHandler(http.FS(s.config.WebFS)))
	} else if s.config.WebRoot != "" {
		// Use filesystem path
		s.echo.GET("/*", s.spaHandler(http.Dir(s.config.WebRoot)))
	}
}

// spaHandler returns a handler for serving Single Page Applications
func (s *Server) spaHandler(fileSystem http.FileSystem) echo.HandlerFunc {
	return func(c *echo.Context) error {
		path := c.Request().URL.Path

		// Skip API, Connect, and Swagger paths
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/sreportal.") || strings.HasPrefix(path, "/swagger") {
			return echo.ErrNotFound
		}

		// Try to serve the file
		file, err := fileSystem.Open(path)
		if err != nil {
			// Fall back to index.html for SPA routing
			path = "/index.html"
			file, err = fileSystem.Open(path)
			if err != nil {
				return echo.ErrNotFound
			}
		}
		defer file.Close() //nolint:errcheck

		stat, err := file.Stat()
		if err != nil {
			return echo.ErrNotFound
		}

		// If it's a directory, try to serve index.html
		if stat.IsDir() {
			indexPath := filepath.Join(path, "index.html")
			indexFile, err := fileSystem.Open(indexPath)
			if err != nil {
				return echo.ErrNotFound
			}
			defer indexFile.Close() //nolint:errcheck
			file = indexFile
			stat, _ = indexFile.Stat()
		}

		// Serve the file
		http.ServeContent(c.Response(), c.Request(), stat.Name(), stat.ModTime(), file)
		return nil
	}
}

// healthHandler returns the health status
func (s *Server) healthHandler(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// Start starts the web server
func (s *Server) Start() error {
	protos := new(http.Protocols)
	protos.SetHTTP1(true)
	protos.SetUnencryptedHTTP2(true)

	s.httpServer = &http.Server{
		Addr:      s.config.Address,
		Handler:   s.echo,
		Protocols: protos,
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// MountHandler registers an external http.Handler on the given path.
// This is useful for mounting additional services (e.g. MCP) on the same port.
func (s *Server) MountHandler(path string, handler http.Handler) {
	s.echo.Any(path, echo.WrapHandler(handler))
}

// statusCaptureWriter wraps http.ResponseWriter to capture the real HTTP status
// code and, for error responses, the response body. This is necessary because
// Connect handlers write status codes directly to the underlying writer,
// bypassing Echo's response tracking.
type statusCaptureWriter struct {
	http.ResponseWriter
	code    int
	errBody []byte // populated only for 4xx/5xx responses
}

func (w *statusCaptureWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusCaptureWriter) Write(b []byte) (int, error) {
	if w.code >= http.StatusBadRequest {
		w.errBody = append(w.errBody, b...)
	}
	return w.ResponseWriter.Write(b)
}

// errorMessage extracts the "message" field from a Connect JSON error body,
// or returns an empty string if unavailable.
func (w *statusCaptureWriter) errorMessage() string {
	if len(w.errBody) == 0 {
		return ""
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.errBody, &body); err != nil {
		return ""
	}
	return body.Message
}

// capturedStatus returns the statusCaptureWriter installed on the Echo context,
// or nil if none is present.
func capturedStatus(c *echo.Context) *statusCaptureWriter {
	sw, _ := c.Response().(*statusCaptureWriter)
	return sw
}

// requestLoggerMiddleware logs every request with the real HTTP status captured
// from the response writer. Log level is chosen by status code:
//
//	2xx/3xx → INFO, 4xx → WARN, 5xx/error → ERROR.
func requestLoggerMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	logger := log.Default().WithName("http")

	return func(c *echo.Context) error {
		start := time.Now()

		// Install the capture writer so all inner middleware and handlers
		// (including Connect) record the real status code.
		sw := &statusCaptureWriter{ResponseWriter: c.Response(), code: http.StatusOK}
		c.SetResponse(sw)

		err := next(c)

		attrs := []any{
			"method", c.Request().Method,
			"uri", c.Request().RequestURI,
			"status", sw.code,
			"latency", time.Since(start).String(),
			"remoteIP", c.RealIP(),
		}

		switch {
		case err != nil:
			logger.Error(err, "request", attrs...)
		case sw.code >= http.StatusInternalServerError:
			logger.Error(nil, "request", append(attrs, "error", sw.errorMessage())...)
		case sw.code >= http.StatusBadRequest:
			logger.Warn("request", append(attrs, "error", sw.errorMessage())...)
		default:
			logger.Info("request", attrs...)
		}

		return err
	}
}

// metricsMiddleware records Prometheus metrics for every HTTP request.
// It relies on the statusCaptureWriter installed by requestLoggerMiddleware.
func metricsMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		metrics.HTTPRequestsInFlight.Inc()
		start := time.Now()

		err := next(c)

		metrics.HTTPRequestsInFlight.Dec()
		duration := time.Since(start).Seconds()

		handler := routeHandler(c)
		method := c.Request().Method

		code := strconv.Itoa(http.StatusOK)
		if sw := capturedStatus(c); sw != nil {
			code = strconv.Itoa(sw.code)
		}

		metrics.HTTPRequestDuration.WithLabelValues(method, handler).Observe(duration)
		metrics.HTTPRequestsTotal.WithLabelValues(method, handler, code).Inc()

		return err
	}
}

// routeHandler returns a low-cardinality handler label for the request.
func routeHandler(c *echo.Context) string {
	path := c.Request().URL.Path
	switch {
	case strings.HasPrefix(path, "/sreportal."):
		return "connect"
	case strings.HasPrefix(path, "/mcp"):
		return "mcp"
	case strings.HasPrefix(path, "/swagger"):
		return "swagger"
	case strings.HasPrefix(path, "/api/"):
		return "api"
	default:
		return "static"
	}
}

// Handler returns the HTTP handler for the server
func (s *Server) Handler() http.Handler {
	return s.echo
}
