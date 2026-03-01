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
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/grpc"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// Config holds the web server configuration
type Config struct {
	// Address is the address to listen on (e.g., ":8080")
	Address string

	// WebRoot is the path to the Angular dist directory
	WebRoot string

	// WebFS is an optional embedded filesystem for the web UI
	WebFS fs.FS
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

	// Middleware
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
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

// setupRoutes configures all routes
func (s *Server) setupRoutes() {
	// Mount Connect handlers for gRPC/Connect protocol
	dnsService := grpc.NewDNSService(s.client)
	dnsPath, dnsHandler := sreportalv1connect.NewDNSServiceHandler(dnsService)
	s.echo.Any(dnsPath+"*", echo.WrapHandler(dnsHandler))

	portalService := grpc.NewPortalService(s.client)
	portalPath, portalHandler := sreportalv1connect.NewPortalServiceHandler(portalService)
	s.echo.Any(portalPath+"*", echo.WrapHandler(portalHandler))

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

		// Skip API and Connect paths
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/sreportal.") {
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

// Handler returns the HTTP handler for the server
func (s *Server) Handler() http.Handler {
	h2s := &http2.Server{}
	return h2c.NewHandler(s.echo, h2s)
}
