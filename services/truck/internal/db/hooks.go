package db

import (
	"time"

	"gorm.io/gorm"

	"go.novek.io/truck/internal/metrics"
)

// RegisterMetricsHooks registers GORM hooks for database metrics
func RegisterMetricsHooks(db *gorm.DB) {
	// Add a callback after database operations to collect metrics

	// For creates
	db.Callback().Create().After("gorm:create").Register("metrics:create", func(db *gorm.DB) {
		collector := metrics.GetMetricsCollector()
		collector.RecordDatabaseQuery(metrics.DBQueryTypeInsert, db.Error == nil, getDuration(db))
	})

	// For queries
	db.Callback().Query().After("gorm:query").Register("metrics:query", func(db *gorm.DB) {
		collector := metrics.GetMetricsCollector()
		collector.RecordDatabaseQuery(metrics.DBQueryTypeSelect, db.Error == nil, getDuration(db))
	})

	// For updates
	db.Callback().Update().After("gorm:update").Register("metrics:update", func(db *gorm.DB) {
		collector := metrics.GetMetricsCollector()
		collector.RecordDatabaseQuery(metrics.DBQueryTypeUpdate, db.Error == nil, getDuration(db))
	})

	// For deletes
	db.Callback().Delete().After("gorm:delete").Register("metrics:delete", func(db *gorm.DB) {
		collector := metrics.GetMetricsCollector()
		collector.RecordDatabaseQuery(metrics.DBQueryTypeDelete, db.Error == nil, getDuration(db))
	})
}

// Get the duration of the database operation
func getDuration(db *gorm.DB) time.Duration {
	if start, ok := db.InstanceGet("start_time"); ok {
		return time.Since(start.(time.Time))
	}
	return 0
}

// LogDuration sets the start time of the database operation
func LogDuration(db *gorm.DB) {
	db.InstanceSet("start_time", time.Now())
}

// Add a callback before database operations to set the start time
func RegisterDurationHooks(db *gorm.DB) {
	db.Callback().Create().Before("gorm:create").Register("duration:create", LogDuration)
	db.Callback().Query().Before("gorm:query").Register("duration:query", LogDuration)
	db.Callback().Update().Before("gorm:update").Register("duration:update", LogDuration)
	db.Callback().Delete().Before("gorm:delete").Register("duration:delete", LogDuration)
}
