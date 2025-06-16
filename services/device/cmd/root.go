package cmd

import (
	"fmt"
	"os"

	"example.com/backstage/services/device/config"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// Used for flags
	cfgFile   string
	logLevel  string
	logFormat string

	// Logger instance for all commands
	log = logrus.New()
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "device-service",
	Short: "Device management service",
	Long: `Device service for managing IoT devices, firmware updates,
and message processing for the backstage platform.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set up logging based on command line flags
		setupLogging()

		// Initialize configuration
		if err := config.InitConfig(cfgFile); err != nil {
			log.Fatalf("Error initializing configuration: %v", err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Initialize root command flags

	// Config file flag
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")

	// Logging flags
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "json", "log format (json, text)")

	// Version flag with short form option
	rootCmd.PersistentFlags().BoolP("version", "v", false, "display version information")

	// Handle version flag if present
	// cobra.OnInitialize(func() {
	// 	if v, _ := rootCmd.PersistentFlags().GetBool("version"); v {
	// 		fmt.Printf("Device Service version %s\n", common.Version)
	// 		os.Exit(0)
	// 	}
	// })
}

// setupLogging configures the global logger based on command line flags
func setupLogging() {
	// Set log level
	switch logLevel {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}

	// Set log format
	if logFormat == "json" {
		log.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	// Output to stderr
	log.SetOutput(os.Stderr)
}
