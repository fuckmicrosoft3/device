package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"example.com/backstage/services/sales/internal/config"
	"example.com/backstage/services/sales/internal/enrichment"
	"example.com/backstage/services/sales/internal/indexer"
	"example.com/backstage/services/sales/internal/store"
	"example.com/backstage/services/sales/pkg/utils"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// Server represents the main server instance
type Server struct {
	config           *config.Config
	store            store.Store
	httpServer       *http.Server
	serviceBusClient *azservicebus.Client
	enricher         *enrichment.Enricher
	indexer          *indexer.Indexer
	idempotencyCache *utils.IdempotencyCache
	messageDedup     *utils.MessageDeduplicator
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config) (*Server, error) {
	// Initialize store
	store, err := store.NewStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	// Initialize enricher
	enricher := enrichment.NewEnricher(store, &cfg.Enrichment)

	// Initialize indexer
	indexer, err := indexer.NewIndexer(cfg, store)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize indexer: %w", err)
	}

	// Initialize idempotency cache
	idempotencyCache, err := utils.NewIdempotencyCache(10000)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize idempotency cache: %w", err)
	}

	// Initialize message deduplicator
	messageDedup := utils.NewMessageDeduplicator(24 * time.Hour)

	// Initialize Azure Service Bus client
	serviceBusClient, err := azservicebus.NewClientFromConnectionString(
		cfg.Azure.ServiceBus.ConnectionString,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize service bus client: %w", err)
	}

	// Create server
	server := &Server{
		config:           cfg,
		store:            store,
		serviceBusClient: serviceBusClient,
		enricher:         enricher,
		indexer:          indexer,
		idempotencyCache: idempotencyCache,
		messageDedup:     messageDedup,
	}

	// Setup HTTP server
	server.setupHTTPServer()

	return server, nil
}

// setupHTTPServer configures the HTTP server and routes
func (s *Server) setupHTTPServer() {
	// Configure Gin
	if s.config.Service.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(s.loggingMiddleware())

	// Health check endpoints
	router.GET("/health", s.handleHealth)
	router.GET("/ready", s.handleReady)

	// API routes
	api := router.Group("/api/v1")
	{
		api.POST("sales/dispense", s.handleDispensePayload)
		api.GET("sales/:id", s.handleGetSale)
		api.GET("/stats", s.handleStats)
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         s.config.Server.HTTP.Address,
		Handler:      router,
		ReadTimeout:  s.config.Server.HTTP.ReadTimeout,
		WriteTimeout: s.config.Server.HTTP.WriteTimeout,
	}
}

// Run starts the server
func (s *Server) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	// Start enrichment service
	s.enricher.Start(ctx)

	// Start indexer
	s.indexer.Start(ctx, 3) // 3 workers

	// Start HTTP server
	g.Go(func() error {
		log.Info().Str("address", s.config.Server.HTTP.Address).Msg("Starting HTTP server")
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server error: %w", err)
		}
		return nil
	})

	// Start Service Bus listener
	g.Go(func() error {
		return s.runServiceBusListener(ctx)
	})

	// Handle graceful shutdown
	g.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("Shutting down server")

		// Shutdown HTTP server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Failed to shutdown HTTP server gracefully")
		}

		// Stop indexer
		s.indexer.Stop()

		// Close Service Bus client
		if err := s.serviceBusClient.Close(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Failed to close Service Bus client")
		}

		return nil
	})

	return g.Wait()
}

// runServiceBusListener runs the Azure Service Bus message listener
func (s *Server) runServiceBusListener(ctx context.Context) error {
	log.Info().
		Str("queue", s.config.Azure.ServiceBus.QueueName).
		Msg("Starting Service Bus listener")

	// Create receiver
	receiver, err := s.serviceBusClient.NewReceiverForQueue(
		s.config.Azure.ServiceBus.QueueName,
		&azservicebus.ReceiverOptions{
			ReceiveMode: azservicebus.ReceiveModePeekLock,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create receiver: %w", err)
	}
	defer receiver.Close(context.Background())

	// Process messages
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// Receive messages
			messages, err := receiver.ReceiveMessages(
				ctx,
				s.config.Azure.ServiceBus.MaxConcurrentMessages,
				&azservicebus.ReceiveMessagesOptions{
					MaxWaitTime: 30 * time.Second,
				},
			)
			if err != nil {
				log.Error().Err(err).Msg("Failed to receive messages")
				time.Sleep(5 * time.Second)
				continue
			}

			// Process messages concurrently
			for _, msg := range messages {
				msg := msg // capture loop variable

				go func() {
					processor := &MessageProcessor{
						server:  s,
						message: msg,
					}

					if err := processor.Process(ctx); err != nil {
						log.Error().
							Err(err).
							Str("message_id", *msg.MessageID).
							Msg("Failed to process message")

						// Abandon message for retry
						if err := receiver.AbandonMessage(ctx, msg, nil); err != nil {
							log.Error().Err(err).Msg("Failed to abandon message")
						}
					} else {
						// Complete message
						if err := receiver.CompleteMessage(ctx, msg, nil); err != nil {
							log.Error().Err(err).Msg("Failed to complete message")
						}
					}
				}()
			}
		}
	}
}

// loggingMiddleware returns a Gin middleware for logging
func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		// Process request
		c.Next()

		// Log request
		latency := time.Since(start)
		status := c.Writer.Status()

		log.Info().
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", status).
			Dur("latency", latency).
			Str("ip", c.ClientIP()).
			Msg("HTTP request")
	}
}
