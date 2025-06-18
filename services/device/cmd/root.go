// services/device/cmd/root.go
package cmd

import (
	"os"

	"example.com/backstage/services/device/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	cfg     *config.Config
	logger  *logrus.Logger
)

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "device-service",
	Short: "IoT Device Management Service",
	Long: `A comprehensive service for managing IoT devices, including
device registration, telemetry ingestion, firmware management, and OTA updates.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config/config.yaml", "config file path")
}

func initConfig() {
	// Initialize logger
	logger = logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Set log level from environment
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		logLevel, err := logrus.ParseLevel(level)
		if err == nil {
			logger.SetLevel(logLevel)
		}
	}

	// Load configuration
	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	cfg.Logger = logger
	logger.Info("Configuration loaded successfully")
}
