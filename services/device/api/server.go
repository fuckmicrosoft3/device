// api/server.go
package api

import (
	"context"
	"fmt"
	"net/http"
	
	"example.com/backstage/services/device/api/middleware"
	"example.com/backstage/services/device/api/routes"
	"example.com/backstage/services/device/config"
	"example.com/backstage/services/device/internal/repository" // Add this import
	"example.com/backstage/services/device/internal/service"
	
	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/sirupsen/logrus"
)

// Server represents the HTTP server
type Server struct {
	router     *gin.Engine
	config     *config.Config
	httpServer *http.Server
	log        *logrus.Logger
}

// NewServer creates a new HTTP server
func NewServer(
	config *config.Config, 
	log *logrus.Logger, 
	nrApp *newrelic.Application,
	svc service.Service,
	repo repository.Repository,
) *Server {
	// Set Gin mode
	gin.SetMode(config.Server.Mode)
	
	// Create router
	router := gin.New()
	
	// Set up middleware
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(log))
	
	// Add New Relic middleware if enabled
	if nrApp != nil {
		router.Use(middleware.NewRelicMiddleware(nrApp))
	}
	
	// Set up routes with repository for auth
	routes.SetupRoutes(router, svc, repo, log)
	
	return &Server{
		router: router,
		config: config,
		log:    log,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", config.Server.Port),
			Handler: router,
		},
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.log.Infof("Starting server on port %d", s.config.Server.Port)
	return s.httpServer.ListenAndServe()
}

// Shutdown stops the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}