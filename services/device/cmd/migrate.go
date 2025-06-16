package cmd

import (
	"example.com/backstage/services/device/config"
	"example.com/backstage/services/device/internal/database"
	
	"github.com/spf13/cobra"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long: `Runs database migrations to ensure the database schema
is up-to-date. This is useful for CI/CD pipelines or initial setup.`,
	Run: func(cmd *cobra.Command, args []string) {
		runMigration()
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

// runMigration executes the database migrations
func runMigration() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	log.Info("Connecting to database...")
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	// Run database migrations
	log.Info("Running database migrations...")
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}
	
	log.Info("Database migrations completed successfully")
}