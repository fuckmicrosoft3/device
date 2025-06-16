package repository

import (
	"context"
	"time"
	
	"example.com/backstage/services/device/internal/database"
	"example.com/backstage/services/device/internal/models"
	
	"gorm.io/gorm"
)

// Repository provides data access methods
type Repository interface {
	// Transaction support
	WithTransaction(ctx context.Context, fn func(ctx context.Context, txRepo Repository) error) error

	// Device operations
	CreateDevice(ctx context.Context, device *models.Device) error
	UpdateDevice(ctx context.Context, device *models.Device) error
	FindDeviceByID(ctx context.Context, id uint) (*models.Device, error)
	FindDeviceByUID(ctx context.Context, uid string) (*models.Device, error)
	ListDevices(ctx context.Context, orgID uint) ([]*models.Device, error)
	
	// DeviceMessage operations
	SaveDeviceMessage(ctx context.Context, message *models.DeviceMessage) error
	FindDeviceMessageByUUID(ctx context.Context, uuid string) (*models.DeviceMessage, error)
	ListDeviceMessages(ctx context.Context, deviceID uint, limit int) ([]*models.DeviceMessage, error)
	MarkMessageAsPublished(ctx context.Context, uuid string) error
	
	// Batch operations - new methods for improved performance
	SaveDeviceMessageBatch(ctx context.Context, messages []*models.DeviceMessage) error
	MarkMessagesAsPublished(ctx context.Context, uuids []string) error
	
	// Organization operations
	CreateOrganization(ctx context.Context, org *models.Organization) error
	UpdateOrganization(ctx context.Context, org *models.Organization) error
	FindOrganizationByID(ctx context.Context, id uint) (*models.Organization, error)
	ListOrganizations(ctx context.Context) ([]*models.Organization, error)
	
	// FirmwareRelease operations
	CreateFirmwareRelease(ctx context.Context, release *models.FirmwareRelease) error
	UpdateFirmwareRelease(ctx context.Context, release *models.FirmwareRelease) error
	FindFirmwareReleaseByID(ctx context.Context, id uint) (*models.FirmwareRelease, error)
	ListFirmwareReleases(ctx context.Context, releaseType models.ReleaseType) ([]*models.FirmwareRelease, error)
	
	// APIKey operations
	CreateAPIKey(ctx context.Context, apiKey *models.APIKey) error
	GetAPIKeyByKey(ctx context.Context, key string) (*models.APIKey, error)
	UpdateAPIKey(ctx context.Context, apiKey *models.APIKey) error
	ListAPIKeys(ctx context.Context) ([]*models.APIKey, error)
	DeleteAPIKey(ctx context.Context, id uint) error
}

// FirmwareRepository provides firmware-specific data access methods
type FirmwareRepository interface {
	GetFirmwareReleaseByID(ctx context.Context, id uint) (*models.FirmwareReleaseExtended, error)
	CreateFirmwareRelease(ctx context.Context, release *models.FirmwareReleaseExtended) error
	UpdateFirmwareRelease(ctx context.Context, release *models.FirmwareReleaseExtended) error
	GetFirmwareReleaseByVersion(ctx context.Context, version string, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error)
	ListFirmwareReleases(ctx context.Context, releaseType models.ReleaseType) ([]*models.FirmwareReleaseExtended, error)
	GetLatestFirmwareRelease(ctx context.Context, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error)
	CreateFirmwareValidation(ctx context.Context, validation *models.FirmwareReleaseValidation) error
	ListValidFirmwareReleases(ctx context.Context, releaseType models.ReleaseType, activeOnly bool) ([]*models.FirmwareReleaseExtended, error)
	GetFirmwareManifest(ctx context.Context, id uint) (*models.FirmwareManifest, error)
	CreateFirmwareManifest(ctx context.Context, manifest *models.FirmwareManifest, releases []*models.FirmwareReleaseExtended) error
	UpdateFirmwareManifest(ctx context.Context, manifest *models.FirmwareManifest) error
}

// OTARepository provides OTA-specific data access methods
type OTARepository interface {
	GetUpdateSession(ctx context.Context, sessionID string) (*models.OTAUpdateSession, error)
	CreateUpdateSession(ctx context.Context, session *models.OTAUpdateSession) error
	UpdateUpdateSession(ctx context.Context, session *models.OTAUpdateSession) error
	ListDeviceUpdateSessions(ctx context.Context, deviceID uint, limit int) ([]*models.OTAUpdateSession, error)
	GetPendingUpdateSessions(ctx context.Context, deviceID uint) ([]*models.OTAUpdateSession, error)
	GetUpdateBatch(ctx context.Context, batchID string) (*models.OTAUpdateBatch, error)
	CreateUpdateBatch(ctx context.Context, batch *models.OTAUpdateBatch) error
	UpdateUpdateBatch(ctx context.Context, batch *models.OTAUpdateBatch) error
	ListBatchUpdateSessions(ctx context.Context, batchID string) ([]*models.OTAUpdateSession, error)
	CreateDeviceLog(ctx context.Context, log *models.OTADeviceLog) error
	GetStuckUpdateSessions(ctx context.Context, threshold time.Time) ([]*models.OTAUpdateSession, error)
	GetUpdateStats(ctx context.Context) (map[string]interface{}, error)
}

// repo is an implementation of the Repository interface
type repo struct {
	db database.DB
}

// Helper type for transaction support
type dbWrapper struct {
	db *gorm.DB
}

func (w *dbWrapper) DB() (*gorm.DB, error) {
	return w.db, nil
}

func (w *dbWrapper) Close() error {
    return nil
}

// NewRepository creates a new repository instance
func NewRepository(db database.DB) Repository {
	return &repo{
		db: db,
	}
}

// firmwareRepo implements FirmwareRepository
type firmwareRepo struct {
	db database.DB
}

// NewFirmwareRepository creates a new firmware repository instance
func NewFirmwareRepository(db database.DB) FirmwareRepository {
	return &firmwareRepo{
		db: db,
	}
}

// otaRepo implements OTARepository
type otaRepo struct {
	db database.DB
}

// NewOTARepository creates a new OTA repository instance
func NewOTARepository(db database.DB) OTARepository {
	return &otaRepo{
		db: db,
	}
}

// WithTransaction executes the given function within a database transaction
func (r *repo) WithTransaction(ctx context.Context, fn func(ctx context.Context, txRepo Repository) error) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	return gormDB.Transaction(func(tx *gorm.DB) error {
		txRepo := &repo{
			db: &dbWrapper{db: tx},
		}
		return fn(ctx, txRepo)
	})
}

// Device operations implementation

func (r *repo) CreateDevice(ctx context.Context, device *models.Device) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	return gormDB.Create(device).Error
}

func (r *repo) UpdateDevice(ctx context.Context, device *models.Device) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	return gormDB.Save(device).Error
}

func (r *repo) FindDeviceByID(ctx context.Context, id uint) (*models.Device, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}
	
	var device models.Device
	if err := gormDB.Preload("Organization").Preload("CurrentRelease").First(&device, id).Error; err != nil {
		return nil, err
	}
	
	return &device, nil
}

func (r *repo) FindDeviceByUID(ctx context.Context, uid string) (*models.Device, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}
	
	var device models.Device
	if err := gormDB.Preload("Organization").Preload("CurrentRelease").Where("uuid = ?", uid).First(&device).Error; err != nil {
		return nil, err
	}
	
	return &device, nil
}

func (r *repo) ListDevices(ctx context.Context, orgID uint) ([]*models.Device, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}
	
	var devices []*models.Device
	query := gormDB.Preload("Organization").Preload("CurrentRelease")
	
	if orgID > 0 {
		query = query.Where("organization_id = ?", orgID)
	}
	
	if err := query.Find(&devices).Error; err != nil {
		return nil, err
	}
	
	return devices, nil
}

// DeviceMessage operations implementation

func (r *repo) SaveDeviceMessage(ctx context.Context, message *models.DeviceMessage) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	return gormDB.Create(message).Error
}

func (r *repo) FindDeviceMessageByUUID(ctx context.Context, uuid string) (*models.DeviceMessage, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}
	
	var message models.DeviceMessage
	if err := gormDB.Preload("Device").Where("uuid = ?", uuid).First(&message).Error; err != nil {
		return nil, err
	}
	
	return &message, nil
}

func (r *repo) ListDeviceMessages(ctx context.Context, deviceID uint, limit int) ([]*models.DeviceMessage, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}
	
	var messages []*models.DeviceMessage
	query := gormDB.Where("device_id = ?", deviceID).Order("created_at DESC")
	
	if limit > 0 {
		query = query.Limit(limit)
	}
	
	if err := query.Find(&messages).Error; err != nil {
		return nil, err
	}
	
	return messages, nil
}

func (r *repo) MarkMessageAsPublished(ctx context.Context, uuid string) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	now := time.Now()
	return gormDB.Model(&models.DeviceMessage{}).
		Where("uuid = ?", uuid).
		Updates(map[string]interface{}{
			"published":    true,
			"published_at": now,
		}).Error
}

// SaveDeviceMessageBatch saves multiple device messages in a single transaction
// This is a new method for improved performance with batch operations
func (r *repo) SaveDeviceMessageBatch(ctx context.Context, messages []*models.DeviceMessage) error {
	if len(messages) == 0 {
		return nil
	}
	
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	// Use transaction for batch insertion
	return gormDB.Transaction(func(tx *gorm.DB) error {
		// Create in batches of 100
		batchSize := 100
		for i := 0; i < len(messages); i += batchSize {
			end := i + batchSize
			if end > len(messages) {
				end = len(messages)
			}
			
			if err := tx.Create(messages[i:end]).Error; err != nil {
				return err
			}
		}
		
		return nil
	})
}

// MarkMessagesAsPublished marks multiple messages as published in a single operation
// This is a new method for improved performance with batch operations
func (r *repo) MarkMessagesAsPublished(ctx context.Context, uuids []string) error {
	if len(uuids) == 0 {
		return nil
	}
	
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	now := time.Now()
	return gormDB.Model(&models.DeviceMessage{}).
		Where("uuid IN ?", uuids).
		Updates(map[string]interface{}{
			"published":    true,
			"published_at": now,
		}).Error
}

// Organization operations implementation

func (r *repo) CreateOrganization(ctx context.Context, org *models.Organization) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	return gormDB.Create(org).Error
}

func (r *repo) UpdateOrganization(ctx context.Context, org *models.Organization) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	return gormDB.Save(org).Error
}

func (r *repo) FindOrganizationByID(ctx context.Context, id uint) (*models.Organization, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}
	
	var org models.Organization
	if err := gormDB.First(&org, id).Error; err != nil {
		return nil, err
	}
	
	return &org, nil
}

func (r *repo) ListOrganizations(ctx context.Context) ([]*models.Organization, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}
	
	var orgs []*models.Organization
	if err := gormDB.Find(&orgs).Error; err != nil {
		return nil, err
	}
	
	return orgs, nil
}

// FirmwareRelease operations implementation

func (r *repo) CreateFirmwareRelease(ctx context.Context, release *models.FirmwareRelease) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	return gormDB.Create(release).Error
}

func (r *repo) UpdateFirmwareRelease(ctx context.Context, release *models.FirmwareRelease) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	return gormDB.Save(release).Error
}

func (r *repo) FindFirmwareReleaseByID(ctx context.Context, id uint) (*models.FirmwareRelease, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}
	
	var release models.FirmwareRelease
	if err := gormDB.Preload("TestRelease").Preload("TestDevice").First(&release, id).Error; err != nil {
		return nil, err
	}
	
	return &release, nil
}

func (r *repo) ListFirmwareReleases(ctx context.Context, releaseType models.ReleaseType) ([]*models.FirmwareRelease, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}
	
	var releases []*models.FirmwareRelease
	query := gormDB.Order("created_at DESC")
	
	if releaseType != "" {
		query = query.Where("release_type = ?", releaseType)
	}
	
	if err := query.Find(&releases).Error; err != nil {
		return nil, err
	}
	
	return releases, nil
}

// APIKey operations implementation
func (r *repo) CreateAPIKey(ctx context.Context, apiKey *models.APIKey) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	return gormDB.Create(apiKey).Error
}

func (r *repo) GetAPIKeyByKey(ctx context.Context, key string) (*models.APIKey, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}
	
	var apiKey models.APIKey
	if err := gormDB.Where("key = ?", key).First(&apiKey).Error; err != nil {
		return nil, err
	}
	
	return &apiKey, nil
}

func (r *repo) UpdateAPIKey(ctx context.Context, apiKey *models.APIKey) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	return gormDB.Save(apiKey).Error
}

func (r *repo) ListAPIKeys(ctx context.Context) ([]*models.APIKey, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}
	
	var apiKeys []*models.APIKey
	if err := gormDB.Find(&apiKeys).Error; err != nil {
		return nil, err
	}
	
	return apiKeys, nil
}

func (r *repo) DeleteAPIKey(ctx context.Context, id uint) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}
	
	return gormDB.Delete(&models.APIKey{}, id).Error
}

