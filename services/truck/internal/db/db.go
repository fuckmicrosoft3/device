package db

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go.novek.io/truck/config"
	"go.novek.io/truck/internal/model"
)

// Connect establishes a connection to the database
func Connect(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode,
	)

	// Configure GORM logger
	var logLevel logger.LogLevel
	if cfg.Debug {
		logLevel = logger.Info
	} else {
		logLevel = logger.Error
	}

	gormLogger := logger.New(
		&logAdapter{},
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	// Open connection
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxConn)
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetConnMaxLifetime(cfg.MaxLife)

	// Register hooks for metrics
	RegisterDurationHooks(db)
	RegisterMetricsHooks(db)

	return db, nil
}

// Migrate runs database migrations
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Device{},
		&model.OperationGroup{},
		&model.Operation{},
		&model.OperationSession{},
		&model.OperationEvent{},
	)
}

// IsRecordNotFoundError checks if an error is a record not found error
func IsRecordNotFoundError(err error) bool {
	return err == gorm.ErrRecordNotFound
}

// logAdapter adapts the GORM logger to the application logger
type logAdapter struct{}

func (l *logAdapter) Printf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}
