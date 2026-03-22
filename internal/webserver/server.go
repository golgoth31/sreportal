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

	"connectrpc.com/connect"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golgoth31/sreportal/internal/log"

	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/grpc"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/openapi"
	releaseservice "github.com/golgoth31/sreportal/internal/release"
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

	// ReleaseService is the shared release service for the ReleaseService gRPC handler
	ReleaseService *releaseservice.Service

	// ReleaseTTL is the TTL for release CRs, exposed to the frontend via ListReleaseDays
	ReleaseTTL time.Duration

	// ReleaseAllowedTypes is the list of allowed release types with display config (empty means all allowed)
	ReleaseAllowedTypes []config.ReleaseTypeConfig
}

// Server is the web server for the SRE Portal
type Server struct {
	config         Config
	echo           *echo.Echo
	client         client.Client
	operatorConfig *config.OperatorConfig
	httpServer     *http.Server
	dnsService     *grpc.DNSService
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

// DNSService returns the underlying DNSService so callers can register it with
// a controller-runtime manager (mgr.Add) to run the shared FQDN cache loop.
func (s *Server) DNSService() *grpc.DNSService {
	return s.dnsService
}

// setupRoutes configures all routes
func (s *Server) setupRoutes() {
	// Shared Connect interceptor — logs handler errors at WARN level since
	// Connect returns HTTP 200 even on coded errors, making them invisible
	// to the Echo request logger middleware.
	connectOpts := connect.WithInterceptors(grpc.LoggingInterceptor())

	// Mount Connect handlers for gRPC/Connect protocol
	s.dnsService = grpc.NewDNSService(s.client)
	dnsPath, dnsHandler := sreportalv1connect.NewDNSServiceHandler(s.dnsService, connectOpts)
	s.echo.Any(dnsPath+"*", echo.WrapHandler(dnsHandler))

	portalService := grpc.NewPortalService(s.client)
	portalPath, portalHandler := sreportalv1connect.NewPortalServiceHandler(portalService, connectOpts)
	s.echo.Any(portalPath+"*", echo.WrapHandler(portalHandler))

	alertmanagerService := grpc.NewAlertmanagerService(s.client)
	amPath, amHandler := sreportalv1connect.NewAlertmanagerServiceHandler(alertmanagerService, connectOpts)
	s.echo.Any(amPath+"*", echo.WrapHandler(amHandler))

	versionService := grpc.NewVersionService()
	versionPath, versionHandler := sreportalv1connect.NewVersionServiceHandler(versionService, connectOpts)
	s.echo.Any(versionPath+"*", echo.WrapHandler(versionHandler))

	if s.config.Gatherer != nil {
		metricsService := grpc.NewMetricsService(s.config.Gatherer)
		metricsPath, metricsHandler := sreportalv1connect.NewMetricsServiceHandler(metricsService, connectOpts)
		s.echo.Any(metricsPath+"*", echo.WrapHandler(metricsHandler))
	}

	if s.config.ReleaseService != nil {
		releaseGRPC := grpc.NewReleaseService(s.config.ReleaseService, s.config.ReleaseTTL, s.config.ReleaseAllowedTypes)
		releasePath, releaseHandler := sreportalv1connect.NewReleaseServiceHandler(releaseGRPC, connectOpts)
		s.echo.Any(releasePath+"*", echo.WrapHandler(releaseHandler))
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
	// Create h2c handler to support HTTP/2 without TLS (for Connect protocol)
	h2s := &http2.Server{}
	handler := h2c.NewHandler(s.echo, h2s)

	s.httpServer = &http.Server{
		Addr:    s.config.Address,
		Handler: handler,
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
	h2s := &http2.Server{}
	return h2c.NewHandler(s.echo, h2s)
}
