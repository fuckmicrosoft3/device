// services/device/cmd/migrate.go
package cmd

import (
	"fmt"

	"example.com/backstage/services/device/internal/core"
	"example.com/backstage/services/device/internal/infrastructure"
	"github.com/spf13/cobra"
)

// migrateCmd represents the migrate command.
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  `Applies all pending database migrations to ensure the schema is up to date.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrations()
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

func runMigrations() error {
	logger.Info("Running database migrations...")

	// Connect to database
	db, err := infrastructure.NewDatabase(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Auto-migrate all models
	logger.Info("Migrating models...")

	models := []interface{}{
		&core.Organization{},
		&core.Device{},
		&core.Telemetry{},
		&core.FirmwareRelease{},
		&core.UpdateSession{},
		&core.AccessToken{},
	}

	for _, model := range models {
		if err := db.DB.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
		logger.Infof("Migrated %T", model)
	}

	// Insert default data if needed
	if err := insertDefaultData(db); err != nil {
		logger.WithError(err).Warn("Failed to insert default data")
	}

	logger.Info("Database migrations completed successfully")
	return nil
}

func insertDefaultData(db *infrastructure.Database) error {
	// Check if we have any organizations
	var count int64
	if err := db.DB.Model(&core.Organization{}).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		logger.Info("Inserting default organizations...")

		// Insert default organizations from config
		defaultOrgs := []core.Organization{
			{
				Name:        "staging.sandbox",
				DisplayName: "Staging Sandbox",
				Environment: core.EnvironmentStaging,
				Active:      true,
				DeviceLimit: 100,
			},
			{
				Name:        "staging.app.ingestor",
				DisplayName: "Staging App Ingestor",
				Environment: core.EnvironmentStaging,
				Active:      true,
				DeviceLimit: 500,
			},
		}

		for _, org := range defaultOrgs {
			if err := db.DB.Create(&org).Error; err != nil {
				logger.WithError(err).WithField("org", org.Name).Warn("Failed to create organization")
			} else {
				logger.WithField("org", org.Name).Info("Created organization")
			}
		}
	}

	// Check if we have any access tokens
	if err := db.DB.Model(&core.AccessToken{}).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		logger.Info("Creating default access tokens...")

		// Create default tokens for testing (only in non-production)
		if cfg.Database.DSN != "" && !isProduction() {
			tokens := []core.AccessToken{
				{
					Token:       "test-admin-token",
					Description: "Admin token for testing",
					Scopes:      []string{"admin"},
				},
				{
					Token:       "test-device-token",
					Description: "Device management token for testing",
					Scopes:      []string{"devices:read", "devices:write", "telemetry:write"},
				},
			}

			for _, token := range tokens {
				if err := db.DB.Create(&token).Error; err != nil {
					logger.WithError(err).Warn("Failed to create test token")
				} else {
					logger.WithField("description", token.Description).Info("Created test token")
				}
			}
		}
	}

	return nil
}

func isProduction() bool {
	// Simple check - you might want to make this more sophisticated
	return cfg.Database.DSN == "" ||
		cfg.Server.Port == 80 ||
		cfg.Server.Port == 443
}
