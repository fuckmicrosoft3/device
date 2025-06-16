package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sales_service",
	Short: "Sales service for processing and recording sales data",
	Long: `A service that processes sales data from Azure Service Bus,
records it in the database, and exposes an API for sales data.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			log.Error().Err(err).Msg("Failed to display help")
		}
	},
}

// Execute executes the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize()
}