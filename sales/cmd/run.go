package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"example.com/backstage/services/sales/internal/config"
	"example.com/backstage/services/sales/internal/server"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the sales service server",
	Long:  `Starts the sales service with both HTTP API and Azure Service Bus message processing.`,
	RunE:  runServer,
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Server specific flags
	runCmd.Flags().String("http-addr", ":8080", "HTTP server address")
	runCmd.Flags().String("queue-name", "", "Azure Service Bus queue name")
	runCmd.Flags().Int("workers", 3, "Number of indexing workers")

	// Bind flags to viper
	viper.BindPFlag("server.http.address", runCmd.Flags().Lookup("http-addr"))
	viper.BindPFlag("azure.service_bus.queue_name", runCmd.Flags().Lookup("queue-name"))
}

func runServer(cmd *cobra.Command, args []string) error {
	// Setup logging
	setupLogging()

	// Load configuration
	cfg, err := loadConfiguration()
	if err != nil {
		return err
	}

	log.Info().
		Str("service", cfg.Service.Name).
		Str("environment", cfg.Service.Environment).
		Msg("Starting sales service")

	// Create server
	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create server")
		return err
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
		cancel()
	}()

	// Run server
	if err := srv.Run(ctx); err != nil {
		log.Error().Err(err).Msg("Server error")
		return err
	}

	log.Info().Msg("Server shutdown complete")
	return nil
}

func setupLogging() {
	// Get log level from config
	logLevel := viper.GetString("logging.level")
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Get log format from config
	logFormat := viper.GetString("logging.format")
	if logFormat == "json" {
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "2006-01-02 15:04:05",
		})
	}

	// Add service context
	log.Logger = log.Logger.With().
		Str("service", "sales-service").
		Logger()
}

func loadConfiguration() (*config.Config, error) {
	// Get config file path
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = "./config.yaml"
	}

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return nil, err
	}

	// Override with command line flags and environment variables
	if viper.IsSet("server.http.address") {
		cfg.Server.HTTP.Address = viper.GetString("server.http.address")
	}

	if viper.IsSet("azure.service_bus.queue_name") {
		cfg.Azure.ServiceBus.QueueName = viper.GetString("azure.service_bus.queue_name")
	}

	if viper.IsSet("service.environment") {
		cfg.Service.Environment = viper.GetString("service.environment")
	}

	return cfg, nil
}
