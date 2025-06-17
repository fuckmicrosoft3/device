package cmd

import (
	"fmt"

	"example.com/backstage/services/device/internal/core"
	"example.com/backstage/services/device/internal/infrastructure"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Runs database migrations",

	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Connecting to database for migration...")
		db, err := infrastructure.NewDatabase(cfg.Database)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}

		logger.Info("Running database migrations...")
		err = db.Migrate(
			&core.Organization{},
			&core.Device{},
			&core.Telemetry{},
			&core.FirmwareRelease{},
			&core.UpdateSession{},
			&core.AccessToken{},
		)
		if err != nil {
			return fmt.Errorf("database migration failed: %w", err)
		}

		logger.Info("Database migration successful.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
