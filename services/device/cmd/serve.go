package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/backstage/services/device/internal/transport/http"
	"example.com/backstage/services/device/internal/core"
	"example.com/backstage/services/device/internal/infrastructure"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts the HTTP server",
	Long:  `Starts the API server to handle device and management requests.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServer()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServer() error {
	logger.Info("Starting server...")

	// --- Infrastructure ---
	db, err := infrastructure.NewDatabase(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	cache, err := infrastructure.NewCache(cfg.Redis)
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	messaging, err := infrastructure.NewMessaging(cfg.ServiceBus)
	if err != nil {
		return fmt.Errorf("failed to connect to Service Bus: %w", err)
	}

	// --- Dependency Injection ---
	repo := core.NewRepository(db.DB)
	service, err := core.NewService(core.ServiceConfig{
		Repository: repo,
		Cache:      cache,
		Messaging:  messaging,
		Logger:     logger,
		FwConfig:   cfg.Firmware,
		OtaConfig:  cfg.OTA,
	})
	if err != nil {
		return fmt.Errorf("failed to create core service: %w", err)
	}

	// --- API Layer & Server ---
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	handlers := api.NewHandlers(service)
	api.SetupRoutes(router, handlers, repo, logger)

	serverAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// --- Graceful Shutdown & Server Start ---
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Infof("Server listening on http://localhost%s", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("FATAL: Could not start server: %v", err)
		}
	}()

	<-stop

	logger.Warn("Shutdown signal received, starting graceful shutdown...")
	service.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Errorf("Server shutdown failed: %v", err)
	} else {
		logger.Info("Server stopped gracefully")
	}
	return nil
}
