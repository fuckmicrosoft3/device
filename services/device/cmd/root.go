package cmd

import (
	"fmt"
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

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "device-service",
	Short: "A service for managing IoT devices and firmware updates.",
	Long: `This service provides a backend for device registration, messaging,
and over-the-air (OTA) firmware updates.`,
	// This will run before any sub-command, ensuring config is always loaded.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize Logger
		logger = logrus.New()
		logger.SetFormatter(&logrus.JSONFormatter{})
		logger.SetOutput(os.Stdout)
		logger.SetLevel(logrus.InfoLevel)

		// Load Config
		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		cfg.Logger = logger
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config/config.yaml", "config file (default is ./config/config.yaml)")
}
