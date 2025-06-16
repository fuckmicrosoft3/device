package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"example.com/backstage/services/canister/eventstore"
	"example.com/backstage/services/canister/models"
	"example.com/backstage/services/canister/projections"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start the projection worker",
	Run:   runWorker,
}

func init() {
	rootCmd.AddCommand(workerCmd)
}

func runWorker(cmd *cobra.Command, args []string) {
    log.Info().Msg("Starting worker")

    // Connect to database
    db, err := gorm.Open(postgres.Open(cfg.DBSource), &gorm.Config{})
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to connect to database")
    }

    // Auto migrate tables
    err = db.AutoMigrate(&models.Event{}, &models.Canister{}, &models.CanisterMovement{})
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to migrate database")
    }

    // Initialize event store
    eventStore := eventstore.NewGormEventStore(db)

    // Initialize Elasticsearch client
    esClient, err := projections.NewElasticsearchClient(cfg)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to initialize Elasticsearch")
    }

    // Initialize projectors
    canisterProjector := projections.NewCanisterProjector(db, esClient, cfg)
    deliveryProjector := projections.NewDeliveryProjector(db, esClient, cfg)

    // Initialize and start event processor
    processor := projections.NewEventProcessor(db, canisterProjector, deliveryProjector, eventStore)
    go processor.Start()

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Info().Msg("Shutting down worker...")

    // Shutdown processor gracefully
    processor.Stop()

    log.Info().Msg("Worker exited properly")
}