package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"example.com/backstage/services/canister/api"
	"example.com/backstage/services/canister/eventstore"
	"example.com/backstage/services/canister/handlers"
	"example.com/backstage/services/canister/messaging"
	"example.com/backstage/services/canister/models"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the API server",
	Run:   runServer,
}

func init() {
	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) {
	log.Info().Msg("Starting server")

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

	// Initialize command handlers
	canisterHandler := handlers.NewCanisterHandler(eventStore)
	deliveryHandler := handlers.NewDeliveryHandler(eventStore)

	// Initialize Azure Service Bus client
	azureClient, err := messaging.NewAzureClient(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Azure Service Bus")
	}

	// Initialize message processor
	msgProcessor := messaging.NewProcessor(canisterHandler, deliveryHandler)

	// Start message consumers
	go func() {
		if err := azureClient.StartConsumers(cfg.AzureMessagesConfigurationQueueName, msgProcessor); err != nil {
			log.Fatal().Err(err).Msg("Failed to start configuration queue consumer")
		}
	}()

	go func() {
		if err := azureClient.StartConsumers(cfg.AzureMessagesEventsQueueName, msgProcessor); err != nil {
			log.Fatal().Err(err).Msg("Failed to start events queue consumer")
		}
	}()

	// Initialize server
	server := api.NewServer(cfg, db, canisterHandler, deliveryHandler)

	// Start HTTP server
	go func() {
		if err := server.Start(); err != nil {
			log.Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown server gracefully
	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exited properly")
}