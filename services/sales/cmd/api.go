package cmd

import (
	"context"
	"os"
	"os/signal"

	"go.novek.io/sales/config"
	"go.novek.io/sales/internal/api"
	"go.novek.io/sales/internal/cache"
	"go.novek.io/sales/internal/metrics"
	"go.novek.io/sales/internal/models"

	// "go.novek.io/sales/internal/repositories"
	"syscall"

	"go.novek.io/sales/internal/search"
	"go.novek.io/sales/internal/services"
	"go.novek.io/sales/internal/tracing"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Start the API server",
	Long:  `Start the HTTP API server to handle incoming sale payloads`,
	RunE:  runAPI,
}

func init() {
	rootCmd.AddCommand(apiCmd)
}

func runAPI(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig(".")
	if err != nil {
		return err
	}

	// Configure logging
	if cfg.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Set up signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Initialize database connections
	db, readOnlyDB, err := initDatabases(cfg)
	if err != nil {
		return err
	}

	// Initialize cache
	redisCache, err := cache.NewRedisCache(cfg.Redis)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize Redis cache, continuing without caching")
	}

	// Initialize tracer
	tracer, err := tracing.NewTracer(cfg.Tracing)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize tracer, continuing without tracing")
	}

	// Initialize Elasticsearch client
	elasticClient, err := search.NewElasticClient(cfg.Elastic)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize Elasticsearch client, continuing without search functionality")
	}

	// Initialize metrics
	metricsCollector := metrics.NewMetrics()

	// Initialize services
	salesService := services.NewSalesService(db, readOnlyDB, redisCache, elasticClient, metricsCollector, tracer)

	// Initialize and start the server
	server := api.NewServer(cfg, salesService, metricsCollector, tracer)

	// Start the server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Error().Err(err).Msg("Server error")
		}
	}()

	// Wait for termination signal
	<-ctx.Done()

	// Shutdown the server
	if err := server.Shutdown(context.Background()); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	}

	log.Info().Msg("Shutting down API server")
	return nil
}

func initDatabases(cfg config.Config) (*gorm.DB, *gorm.DB, error) {
	// Initialize write database
	db, err := gorm.Open(postgres.Open(cfg.DB.DSN), &gorm.Config{})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect to write database")
	}

	// Initialize read-only database
	readOnlyDB, err := gorm.Open(postgres.Open(cfg.DB.ReadOnlyDSN), &gorm.Config{})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect to read-only database")
	}

	// Auto-migrate only the write database
	if err := models.SetupModels(db); err != nil {
		return nil, nil, errors.Wrap(err, "failed to run migrations")
	}

	return db, readOnlyDB, nil
}
