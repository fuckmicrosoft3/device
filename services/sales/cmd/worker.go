package cmd

import (
	"context"
	"os"
	"os/signal"
	"example.com/backstage/services/sales/config"
	"example.com/backstage/services/sales/internal/cache"
	"example.com/backstage/services/sales/internal/messaging"
	"example.com/backstage/services/sales/internal/metrics"
	"example.com/backstage/services/sales/internal/models"
	// "example.com/backstage/services/sales/internal/repositories"
	"example.com/backstage/services/sales/internal/search"
	"example.com/backstage/services/sales/internal/services"
	"example.com/backstage/services/sales/internal/tracing"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start the background worker",
	Long:  `Start the background worker to process messages from Azure Service Bus and reconcile sales`,
	RunE:  runWorker,
}

func init() {
	rootCmd.AddCommand(workerCmd)
}

func runWorker(cmd *cobra.Command, args []string) error {
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

	// Create an error group to manage goroutines
	g, ctx := errgroup.WithContext(ctx)

	// Initialize database connections
	db, readOnlyDB, err := initDatabasesForWorker(cfg)
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

	// Initialize Azure Service Bus client
	azureBus, err := messaging.NewAzureServiceBus(cfg.Azure, tracer)
	if err != nil {
		return err
	}

	// Start the service bus processor
	g.Go(func() error {
		log.Info().Str("queue", cfg.Azure.QueueName).Msg("Starting Azure Service Bus processor")
		return azureBus.ProcessMessages(ctx, salesService.ProcessDispenseMessage)
	})

	// Start the sales reconciliation cron job as a fallback mechanism
	g.Go(func() error {
		log.Info().Msg("Starting sales reconciliation cron job as fallback mechanism")
		
		// Create a scheduler
		scheduler, err := gocron.NewScheduler()
		if err != nil {
			return err
		}
		
		// Add the reconciliation job to run every 5 minutes
		// This is less frequent since it's just a fallback mechanism now
		_, err = scheduler.NewJob(
			gocron.DurationJob(5*time.Minute),
			gocron.NewTask(func() {
				log.Info().Msg("Running fallback reconciliation job to catch any missed sessions")
				if err := salesService.ReconcileSales(ctx); err != nil {
					log.Error().Err(err).Msg("Failed to reconcile sales in fallback job")
				}
			}),
		)
		if err != nil {
			return err
		}
		
		// Start the scheduler
		scheduler.Start()
		
		// Wait for context cancellation
		<-ctx.Done()
		
		// Shutdown the scheduler
		return scheduler.Shutdown()
	})

	// Wait for any goroutine to exit
	if err := g.Wait(); err != nil {
		log.Error().Err(err).Msg("Worker error")
		return err
	}

	log.Info().Msg("Worker shutting down gracefully")
	return nil
}

func initDatabasesForWorker(cfg config.Config) (*gorm.DB, *gorm.DB, error) {
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

	// Configure connection pools for both databases
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get underlying write DB connection")
	}

	// Set connection pool parameters for write DB
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Configure read-only connection pool
	readSqlDB, err := readOnlyDB.DB()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get underlying read-only DB connection")
	}

	// Set connection pool parameters for read-only DB (higher limits for read operations)
	readSqlDB.SetMaxIdleConns(20)
	readSqlDB.SetMaxOpenConns(100)
	readSqlDB.SetConnMaxLifetime(time.Hour)

	return db, readOnlyDB, nil
}