package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"example.com/backstage/services/truck/config"
	"example.com/backstage/services/truck/internal/api"
	"example.com/backstage/services/truck/internal/cache"
	"example.com/backstage/services/truck/internal/db"
	"example.com/backstage/services/truck/internal/messagebus"
	"example.com/backstage/services/truck/internal/repository"
	"example.com/backstage/services/truck/internal/service"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP server",
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			logrus.Fatalf("Failed to load configuration: %v", err)
		}

		// Setup logger
		logger := logrus.New()
		if cfg.Logging.JSON {
			logger.SetFormatter(&logrus.JSONFormatter{})
		} else {
			logger.SetFormatter(&logrus.TextFormatter{
				FullTimestamp: true,
			})
		}

		// Set log level
		level, err := logrus.ParseLevel(cfg.Logging.Level)
		if err != nil {
			level = logrus.InfoLevel
		}
		logger.SetLevel(level)

		// Connect to database
		dbConn, err := db.Connect(&cfg.Database)
		if err != nil {
			logger.Fatalf("Failed to connect to database: %v", err)
		}

		// Run migrations
		if err := db.Migrate(dbConn); err != nil {
			logger.Fatalf("Failed to run database migrations: %v", err)
		}

		// Initialize cache
		cacheClient, err := cache.NewRedisClient(&cfg.Redis)
		if err != nil {
			logger.Fatalf("Failed to connect to Redis: %v", err)
		}

		// Initialize message bus
		messageBusClient, err := messagebus.NewClient(&cfg.MessageBus)
		if err != nil {
			logger.Fatalf("Failed to initialize message bus: %v", err)
		}

		// Create repositories
		deviceRepo := repository.NewDeviceRepository(dbConn)
		operationRepo := repository.NewOperationRepository(dbConn)
		operationGroupRepo := repository.NewOperationGroupRepository(dbConn)
		operationEventRepo := repository.NewOperationEventRepository(dbConn)

		// Create services
		operationService := service.NewOperationService(
			operationRepo,
			deviceRepo,
			messageBusClient,
			cacheClient,
			cfg.MessageBus.ERPQueue,
		)
		operationGroupService := service.NewOperationGroupService(
			operationGroupRepo,
			deviceRepo,
			cacheClient,
		)
		operationEventService := service.NewOperationEventService(
			operationService,
			operationGroupService,
			operationEventRepo,
			deviceRepo,
		)

		// Create API handler
		handler := api.NewHandler(
			operationService,
			operationGroupService,
			operationEventService,
		)

		// Create middleware
		middleware := api.NewMiddleware(logger)

		// Create router
		router := mux.NewRouter()
		
		// Apply middleware
		router.Use(middleware.Logger)
		router.Use(middleware.Recover)
		router.Use(middleware.CORS(cfg.Server.CorsWhiteList))
		router.Use(api.MetricsMiddleware)

		// Register routes
		handler.RegisterRoutes(router.PathPrefix("/api/v1").Subrouter())

		// Setup server
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		server := &http.Server{
			Addr:         addr,
			Handler:      router,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		}

		// Start server in a goroutine
		go func() {
			logger.Infof("Starting server on %s", addr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Fatalf("Failed to start server: %v", err)
			}
		}()

		// Wait for interrupt signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		// Create context with timeout for graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		// Shutdown server
		logger.Info("Shutting down server...")
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Fatalf("Server shutdown failed: %v", err)
		}

		// Close message bus
		if err := messageBusClient.Close(shutdownCtx); err != nil {
			logger.Fatalf("Message bus closure failed: %v", err)
		}

		logger.Info("Server shutdown complete")
	},
}