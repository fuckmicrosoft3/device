// internal/database/database.go
package database

import (
	"fmt"
	"time"
	
	"example.com/backstage/services/device/config"
	"example.com/backstage/services/device/internal/models"
	
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is an interface for database operations
type DB interface {
	DB() (*gorm.DB, error)
	Close() error
}

// GormDatabase implements the DB interface for GORM
type GormDatabase struct {
	db *gorm.DB
}

// Connect establishes a connection to the database
func Connect(cfg config.DatabaseConfig) (DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		DisableForeignKeyConstraintWhenMigrating: true, // Disable foreign key constraints during migration
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get DB instance: %w", err)
	}
	
	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool
	// Increased from 10 to 50 to better handle high concurrency loads
	sqlDB.SetMaxIdleConns(50)
	
	// SetMaxOpenConns sets the maximum number of open connections to the database
	// Increased from 100 to 500 to support higher concurrent processing
	sqlDB.SetMaxOpenConns(500)
	
	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused
	// Reduced from 1 hour to 30 minutes to prevent stale connections
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	
	return &GormDatabase{db: db}, nil
}

// DB returns the underlying gorm.DB instance
func (d *GormDatabase) DB() (*gorm.DB, error) {
	return d.db, nil
}

// Close closes the database connection
func (d *GormDatabase) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// AutoMigrate runs database migrations in a safe order that handles circular dependencies
func AutoMigrate(db DB) error {
	gormDB, err := db.DB()
	if err != nil {
		return err
	}
	
	// Step 1: Disable foreign key constraint checks during migration
	gormDB.DisableForeignKeyConstraintWhenMigrating = true
	
	// Step 2: Migrate all table structures without foreign key constraints
	err = gormDB.AutoMigrate(
		&models.Organization{},
		&models.FirmwareRelease{},
		&models.Device{},
		&models.DeviceMessage{},
		&models.APIKey{}, // Add API Keys table
	)
	
	if err != nil {
		return fmt.Errorf("failed to migrate table structures: %w", err)
	}
	
	// Step 3: Enable foreign key constraint checks for future operations
	gormDB.DisableForeignKeyConstraintWhenMigrating = false
	
	return nil
}