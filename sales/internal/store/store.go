package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"example.com/backstage/services/sales/internal/config"
	"example.com/backstage/services/sales/internal/models"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	ErrNotFound         = errors.New("record not found")
	ErrDuplicateKey     = errors.New("duplicate key violation")
	ErrInvalidOperation = errors.New("invalid operation")
)

// Store interface defines all database operations
type Store interface {
	// Sales database operations
	CreateDispenseSession(ctx context.Context, session *models.DispenseSession) error
	GetDispenseSessionByID(ctx context.Context, id uuid.UUID) (*models.DispenseSession, error)
	GetDispenseSessionByIdempotencyKey(ctx context.Context, key uuid.UUID) (*models.DispenseSession, error)
	GetUnprocessedDispenseSessions(ctx context.Context, limit int) ([]*models.DispenseSession, error)
	GetUnenrichedDispenseSessions(ctx context.Context, limit int) ([]*models.DispenseSession, error)
	UpdateDispenseSession(ctx context.Context, session *models.DispenseSession) error

	CreateSale(ctx context.Context, sale *models.Sale) error
	GetSaleByID(ctx context.Context, id uuid.UUID) (*models.Sale, error)
	GetSaleByDispenseSessionID(ctx context.Context, dispenseSessionID uuid.UUID) (*models.Sale, error)

	// ERP database operations (read-only)
	GetDeviceByMCU(ctx context.Context, mcu string) (*models.Device, error)
	GetActiveDeviceMachineRevision(ctx context.Context, deviceID uuid.UUID, saleTime time.Time) (*models.DeviceMachineRevision, error)
	GetActiveMachineRevision(ctx context.Context, deviceMachineRevisionID uuid.UUID, saleTime time.Time) (*models.MachineRevision, error)
	GetMachineByID(ctx context.Context, id uuid.UUID) (*models.Machine, error)
	GetMachineLocationByMachineID(ctx context.Context, machineID uuid.UUID) (*models.Location, error)
	GetPositionByMachineID(ctx context.Context, machineID uuid.UUID) (*models.Position, error)
	GetTenantByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error)

	// Transaction operations
	WithTransaction(ctx context.Context, fn func(tx Store) error) error

	// Health check
	Ping(ctx context.Context) error
}

// StoreImpl implements the Store interface
type StoreImpl struct {
	salesDB *gorm.DB
	erpDB   *gorm.DB
}

// NewStore creates a new store instance
func NewStore(cfg *config.Config) (Store, error) {
	// Configure GORM logger
	gormLogger := logger.Default.LogMode(logger.Silent)
	if cfg.Logging.Level == "debug" {
		gormLogger = logger.Default.LogMode(logger.Info)
	}

	// Connect to Sales database
	salesDB, err := connectDB(&cfg.Database.Sales, gormLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sales database: %w", err)
	}

	// Connect to ERP database (read-only)
	erpDB, err := connectDB(&cfg.Database.ERP, gormLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ERP database: %w", err)
	}

	store := &StoreImpl{
		salesDB: salesDB,
		erpDB:   erpDB,
	}

	// Run migrations for sales database only
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return store, nil
}

// connectDB establishes a database connection
func connectDB(cfg *config.DatabaseConnection, gormLogger logger.Interface) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger: gormLogger,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return db, nil
}

// Sales database operations

func (s *StoreImpl) CreateDispenseSession(ctx context.Context, session *models.DispenseSession) error {
	result := s.salesDB.WithContext(ctx).Create(session)
	if result.Error != nil {
		if isDuplicateKeyError(result.Error) {
			return ErrDuplicateKey
		}
		return result.Error
	}
	return nil
}

func (s *StoreImpl) GetDispenseSessionByID(ctx context.Context, id uuid.UUID) (*models.DispenseSession, error) {
	var session models.DispenseSession
	result := s.salesDB.WithContext(ctx).First(&session, "id = ?", id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}
	return &session, nil
}

func (s *StoreImpl) GetDispenseSessionByIdempotencyKey(ctx context.Context, key uuid.UUID) (*models.DispenseSession, error) {
	var session models.DispenseSession
	result := s.salesDB.WithContext(ctx).First(&session, "idempotency_key = ?", key)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}
	return &session, nil
}

func (s *StoreImpl) GetUnprocessedDispenseSessions(ctx context.Context, limit int) ([]*models.DispenseSession, error) {
	var sessions []*models.DispenseSession
	result := s.salesDB.WithContext(ctx).
		Where("is_processed = ? AND time IS NOT NULL", false).
		Order("created_at ASC").
		Limit(limit).
		Find(&sessions)

	if result.Error != nil {
		return nil, result.Error
	}
	return sessions, nil
}

func (s *StoreImpl) GetUnenrichedDispenseSessions(ctx context.Context, limit int) ([]*models.DispenseSession, error) {
	var sessions []*models.DispenseSession
	result := s.salesDB.WithContext(ctx).
		Where("enrichment_status = ? AND enrichment_attempts < ?", models.EnrichmentStatusPending, 3).
		Order("created_at ASC").
		Limit(limit).
		Find(&sessions)

	if result.Error != nil {
		return nil, result.Error
	}
	return sessions, nil
}

func (s *StoreImpl) UpdateDispenseSession(ctx context.Context, session *models.DispenseSession) error {
	result := s.salesDB.WithContext(ctx).Save(session)
	return result.Error
}

func (s *StoreImpl) CreateSale(ctx context.Context, sale *models.Sale) error {
	result := s.salesDB.WithContext(ctx).Create(sale)
	if result.Error != nil {
		if isDuplicateKeyError(result.Error) {
			return ErrDuplicateKey
		}
		return result.Error
	}
	return nil
}

func (s *StoreImpl) GetSaleByID(ctx context.Context, id uuid.UUID) (*models.Sale, error) {
	var sale models.Sale
	result := s.salesDB.WithContext(ctx).
		Preload("Machine").
		Preload("MachineRevision").
		First(&sale, "id = ?", id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}
	return &sale, nil
}

func (s *StoreImpl) GetSaleByDispenseSessionID(ctx context.Context, dispenseSessionID uuid.UUID) (*models.Sale, error) {
	var sale models.Sale
	result := s.salesDB.WithContext(ctx).First(&sale, "dispense_session_id = ?", dispenseSessionID)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}
	return &sale, nil
}

// ERP database operations (read-only)

func (s *StoreImpl) GetDeviceByMCU(ctx context.Context, mcu string) (*models.Device, error) {
	var device models.Device
	result := s.erpDB.WithContext(ctx).First(&device, "mcu = ?", mcu)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}
	return &device, nil
}

func (s *StoreImpl) GetActiveDeviceMachineRevision(ctx context.Context, deviceID uuid.UUID, saleTime time.Time) (*models.DeviceMachineRevision, error) {
	var revision models.DeviceMachineRevision
	result := s.erpDB.WithContext(ctx).
		Where("device_id = ? AND active = ? AND start <= ?", deviceID, true, saleTime).
		Where("termination IS NULL OR termination > ?", saleTime).
		First(&revision)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}
	return &revision, nil
}

func (s *StoreImpl) GetActiveMachineRevision(ctx context.Context, deviceMachineRevisionID uuid.UUID, saleTime time.Time) (*models.MachineRevision, error) {
	var revision models.MachineRevision
	result := s.erpDB.WithContext(ctx).
		Where("device_machine_revision_id = ? AND start <= ?", deviceMachineRevisionID, saleTime).
		Where("terminate IS NULL OR terminate > ?", saleTime).
		First(&revision)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}
	return &revision, nil
}

func (s *StoreImpl) GetMachineByID(ctx context.Context, id uuid.UUID) (*models.Machine, error) {
	var machine models.Machine
	result := s.erpDB.WithContext(ctx).First(&machine, "id = ?", id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}
	return &machine, nil
}

func (s *StoreImpl) GetMachineLocationByMachineID(ctx context.Context, machineID uuid.UUID) (*models.Location, error) {
	var location models.Location
	result := s.erpDB.WithContext(ctx).First(&location, "machine_id = ?", machineID)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}
	return &location, nil
}

func (s *StoreImpl) GetPositionByMachineID(ctx context.Context, machineID uuid.UUID) (*models.Position, error) {
	var position models.Position
	result := s.erpDB.WithContext(ctx).First(&position, "machine_id = ?", machineID)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}
	return &position, nil
}

func (s *StoreImpl) GetTenantByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error) {
	var tenant models.Tenant
	result := s.erpDB.WithContext(ctx).First(&tenant, "id = ?", id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}
	return &tenant, nil
}

// Transaction support

func (s *StoreImpl) WithTransaction(ctx context.Context, fn func(tx Store) error) error {
	return s.salesDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txStore := &StoreImpl{
			salesDB: tx,
			erpDB:   s.erpDB, // ERP is read-only, no transaction needed
		}
		return fn(txStore)
	})
}

// Health check

func (s *StoreImpl) Ping(ctx context.Context) error {
	// Check sales database
	sqlDB, err := s.salesDB.DB()
	if err != nil {
		return fmt.Errorf("sales db error: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("sales db ping failed: %w", err)
	}

	// Check ERP database
	sqlDB, err = s.erpDB.DB()
	if err != nil {
		return fmt.Errorf("erp db error: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("erp db ping failed: %w", err)
	}

	return nil
}

// Helper functions

func isDuplicateKeyError(err error) bool {
	errStr := err.Error()
	return gorm.ErrDuplicatedKey.Error() == errStr ||
		contains(errStr, "duplicate key") ||
		contains(errStr, "unique constraint") ||
		contains(errStr, "UNIQUE constraint failed")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}
