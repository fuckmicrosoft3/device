package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go.novek.io/truck/config"
	"go.novek.io/truck/internal/db"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			logrus.Fatalf("Failed to load configuration: %v", err)
		}

		// Connect to database
		dbConn, err := db.Connect(&cfg.Database)
		if err != nil {
			logrus.Fatalf("Failed to connect to database: %v", err)
		}

		// Run migrations
		logrus.Info("Running database migrations...")
		if err := db.Migrate(dbConn); err != nil {
			logrus.Fatalf("Failed to run database migrations: %v", err)
		}

		logrus.Info("Database migrations completed successfully")
	},
}
