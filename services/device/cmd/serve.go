package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/backstage/services/device/internal/api"
	"example.com/backstage/services/device/internal/core"
	"example.com/backstage/services/device/internal/infrastructure"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts the Device Management API server",
	Long:  `Launches the HTTP server to handle device registration, telemetry ingestion, and OTA updates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServer()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServer() error {
	logger.Info("Initializing Device Management Service...")

	// --- Infrastructure Setup ---
	logger.Info("Connecting to database...")
	db, err := infrastructure.NewDatabase(cfg.Database)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	logger.Info("Connecting to cache...")
	cache, err := infrastructure.NewCache(cfg.Redis)
	if err != nil {
		return fmt.Errorf("cache connection failed: %w", err)
	}
	defer cache.Close()

	logger.Info("Connecting to messaging service...")
	messaging, err := infrastructure.NewMessaging(cfg.ServiceBus)
	if err != nil {
		logger.Warn("Messaging service unavailable, continuing without it")
		messaging = nil
	} else {
		defer messaging.Close()
	}

	// --- Service Layer Setup ---
	dataStore := core.NewDataStore(db.DB)

	serviceConfig := core.ServiceConfig{
		DataStore:      dataStore,
		Cache:          cache,
		Messaging:      messaging,
		Logger:         logger,
		FirmwareConfig: cfg.Firmware,
		OTAConfig:      cfg.OTA,
	}

	services, err := core.NewServiceRegistry(serviceConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize services: %w", err)
	}

	// --- API Layer Setup ---
	if gin.Mode() == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	handlers := api.NewAPIHandlers(services)
	api.SetupRoutes(router, handlers, services, logger)

	// --- HTTP Server ---
	serverAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// --- Graceful Shutdown ---
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Infof("Device Management API listening on %s", serverAddr)
		logger.Info("Service started successfully")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-shutdownChan

	logger.Warn("Shutdown signal received, initiating graceful shutdown...")

	// Shutdown telemetry processor first
	if services.Telemetry != nil {
		if telemetryImpl, ok := services.Telemetry.(*telemetryService); ok {
			telemetryImpl.processor.Stop()
		}
	}

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Errorf("Server shutdown failed: %v", err)
	} else {
		logger.Info("Server stopped gracefully")
	}

	logger.Info("Device Management Service shutdown complete")
	return nil
}
