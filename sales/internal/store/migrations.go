package store

import (
	"example.com/backstage/services/sales/internal/models"

	"github.com/rs/zerolog/log"
)

// migrate runs database migrations for the sales database
func (s *StoreImpl) migrate() error {
	// Auto-migrate sales database models
	err := s.salesDB.AutoMigrate(
		&models.DispenseSession{},
		&models.Sale{},
		&models.Tenant{},
		&models.Organization{},
		&models.User{},
		&models.Product{},
		&models.Transaction{},
	)

	if err != nil {
		return err
	}

	// Create indexes for better performance
	if err := s.createIndexes(); err != nil {
		return err
	}

	log.Info().Msg("Database migrations completed successfully")
	return nil
}

// createIndexes creates custom indexes for performance
func (s *StoreImpl) createIndexes() error {
	// DispenseSession indexes
	if err := s.salesDB.Exec("CREATE INDEX IF NOT EXISTS idx_dispense_sessions_device_mcu ON dispense_sessions(device_mcu)").Error; err != nil {
		return err
	}

	if err := s.salesDB.Exec("CREATE INDEX IF NOT EXISTS idx_dispense_sessions_is_processed ON dispense_sessions(is_processed) WHERE is_processed = false").Error; err != nil {
		return err
	}

	if err := s.salesDB.Exec("CREATE INDEX IF NOT EXISTS idx_dispense_sessions_enrichment ON dispense_sessions(enrichment_status, enrichment_attempts) WHERE enrichment_status = 'PENDING'").Error; err != nil {
		return err
	}

	// Sale indexes
	if err := s.salesDB.Exec("CREATE INDEX IF NOT EXISTS idx_sales_machine_id ON sales(machine_id)").Error; err != nil {
		return err
	}

	if err := s.salesDB.Exec("CREATE INDEX IF NOT EXISTS idx_sales_tenant_id ON sales(tenant_id)").Error; err != nil {
		return err
	}

	if err := s.salesDB.Exec("CREATE INDEX IF NOT EXISTS idx_sales_time ON sales(time)").Error; err != nil {
		return err
	}

	if err := s.salesDB.Exec("CREATE INDEX IF NOT EXISTS idx_sales_type ON sales(type)").Error; err != nil {
		return err
	}

	return nil
}
