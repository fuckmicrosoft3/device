package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"go.novek.io/canister/config"
	"go.novek.io/canister/handlers"
)

// Server is the HTTP server for the API
type Server struct {
	cfg             config.Config
	router          *gin.Engine
	httpServer      *http.Server
	db              *gorm.DB
	canisterHandler *handlers.CanisterHandler
	deliveryHandler *handlers.DeliveryHandler
}

// NewServer creates a new API server
func NewServer(cfg config.Config, db *gorm.DB, canisterHandler *handlers.CanisterHandler, deliveryHandler *handlers.DeliveryHandler) *Server {
	server := &Server{
		cfg:             cfg,
		router:          gin.Default(),
		db:              db,
		canisterHandler: canisterHandler,
		deliveryHandler: deliveryHandler,
	}

	// Setup middleware
	server.setupMiddleware()

	// Setup routes
	server.setupRoutes()

	return server
}

// setupMiddleware adds middleware to the router
func (s *Server) setupMiddleware() {
	// Add request ID middleware
	s.router.Use(RequestIDMiddleware())

	// Add CORS middleware
	s.router.Use(CORSMiddleware())

	// Add recovery middleware
	s.router.Use(gin.Recovery())

	// Add logging middleware
	s.router.Use(LoggingMiddleware())
}

// setupRoutes defines the API routes
func (s *Server) setupRoutes() {
	// Health check
	s.router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// API v1 group
	v1 := s.router.Group("/api/v1")

	// Canister routes
	canisterRoutes := v1.Group("/canister")
	{
		canisterRoutes.POST("/events", s.receiveCanisterEvents)
		canisterRoutes.GET("/:id", s.getCanisterAggregate)
		canisterRoutes.GET("/distribution", s.getOrgCanistersForDistribution)
	}

	// Delivery routes
	deliveryRoutes := v1.Group("/delivery")
	{
		deliveryRoutes.POST("/events", s.receiveDeliveryEvents)
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:    s.cfg.HTTPServerAddress,
		Handler: s.router,
	}

	log.Info().Msgf("HTTP server starting on %s", s.cfg.HTTPServerAddress)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
