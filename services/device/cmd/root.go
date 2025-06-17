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

var rootCmd = &cobra.Command{
	Use:   "device-service",
	Short: "A service for managing devices and firmware updates.",

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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config/config.yaml", "config file (default is ./config/config.yaml)")
}
