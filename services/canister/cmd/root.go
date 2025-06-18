package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.novek.io/canister/config"
)

var (
	cfgFile string
	cfg     config.Config
)

var rootCmd = &cobra.Command{
	Use:   "canister-service",
	Short: "Canister tracking service using event sourcing",
	Long:  `A service for tracking canisters using event sourcing and CQRS pattern`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./app.env)")
}

func initConfig() {
	var err error

	if cfgFile != "" {
		// Use config file from the flag
		config.SetConfigFile(cfgFile)
	}

	cfg, err = config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}
}
