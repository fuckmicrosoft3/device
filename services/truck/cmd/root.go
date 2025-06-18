package cmd

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// Flags
	cfgFile string
	debug   bool

	// Root command
	rootCmd = &cobra.Command{
		Use:   "operations-service",
		Short: "Operations Service",
		Long: `Operations Service for managing field operations between devices, trucks, and ERP.

Functions:
- Receive scheduled operations from the cloud ERP
- Deliver latest information about refills over a REST HTTP server
- Receive latest operational events from trucks and machines from the field
- Publish operational updates to the cloud ERP`,
	}
)

// Execute executes the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")

	// Add commands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(readEventsCmd)
	rootCmd.AddCommand(republishEventsCmd)
}

// initConfig initializes the configuration
func initConfig() {
	// Setup logging
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}
