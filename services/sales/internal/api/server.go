package api

import (
	"context"
	"net/http"
	"example.com/backstage/services/sales/config"
	"example.com/backstage/services/sales/internal/api/handlers"
	"example.com/backstage/services/sales/internal/services"
	"example.com/backstage/services/sales/internal/tracing"
	"example.com/backstage/services/sales/internal/metrics"

	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Server represents the HTTP server
type Server struct {
	config       config.Config
	router       *gin.Engine
	httpServer   *http.Server
	salesService *services.SalesService
	metrics      *metrics.Metrics
	tracer       tracing.Tracer
}

// NewServer creates a new HTTP server
func NewServer(cfg config.Config, salesService *services.SalesService, metrics *metrics.Metrics, tracer tracing.Tracer) *Server {
	server := &Server{
		config:       cfg,
		salesService: salesService,
		metrics:      metrics,
		tracer:       tracer,
	}

	router := server.setupRouter()
	server.router = router

	httpServer := &http.Server{
		Addr:    cfg.HTTPServerAddress,
		Handler: router,
	}
	server.httpServer = httpServer

	return server
}

// setupRouter configures the HTTP router
func (s *Server) setupRouter() *gin.Engine {
	router := gin.Default()

	// Recovery middleware
	router.Use(gin.Recovery())

	// Register handlers
	salesHandler := handlers.NewSalesHandler(s.salesService, s.tracer)
	salesHandler.RegisterRoutes(router)

	// Register metrics handler if enabled
	if s.config.MetricsEnabled {
		metricsHandler := handlers.NewMetricsHandler(s.metrics, s.tracer)
		metricsHandler.RegisterRoutes(router)
	}

	return router
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Info().Str("address", s.config.HTTPServerAddress).Msg("Starting HTTP server")

	if err := s.httpServer.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.Wrap(err, "HTTP server error")
	}

	return nil
}

// Shutdown gracefully stops the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Info().Msg("Shutting down HTTP server")

	// Create a timeout context for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return errors.Wrap(err, "HTTP server shutdown error")
	}

	log.Info().Msg("HTTP server shut down successfully")
	return nil
}