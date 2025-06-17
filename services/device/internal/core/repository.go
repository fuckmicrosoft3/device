// services/device/internal/core/repository.go
package core

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// DataStore defines all data access operations
type DataStore interface {
	// Device operations
	CreateDevice(ctx context.Context, device *Device) error
	UpdateDevice(ctx context.Context, device *Device) error
	GetDevice(ctx context.Context, id uint) (*Device, error)
	GetDeviceByUID(ctx context.Context, uid string) (*Device, error)
	ListDevicesByOrganization(ctx context.Context, orgID uint) ([]*Device, error)
	UpdateDeviceHeartbeat(ctx context.Context, deviceID uint) error

	// Telemetry operations
	SaveTelemetry(ctx context.Context, telemetry *Telemetry) error
	SaveTelemetryBatch(ctx context.Context, telemetry []*Telemetry) error
	MarkTelemetryProcessed(ctx context.Context, messageID string) error
	GetDeviceTelemetry(ctx context.Context, deviceID uint, limit int) ([]*Telemetry, error)
	GetUnprocessedTelemetry(ctx context.Context, limit int) ([]*Telemetry, error)
	GetTelemetryByTimeRange(ctx context.Context, deviceID uint, start, end time.Time) ([]*Telemetry, error)
	GetFailedTelemetry(ctx context.Context, limit int) ([]*Telemetry, error)
	UpdateTelemetryRetryCount(ctx context.Context, messageID string, retryCount int, lastError string) error

	// Organization operations
	CreateOrganization(ctx context.Context, org *Organization) error
	UpdateOrganization(ctx context.Context, org *Organization) error
	GetOrganization(ctx context.Context, id uint) (*Organization, error)
	ListOrganizations(ctx context.Context) ([]*Organization, error)

	// Firmware operations
	CreateFirmwareRelease(ctx context.Context, firmware *FirmwareRelease) error
	UpdateFirmwareRelease(ctx context.Context, firmware *FirmwareRelease) error
	GetFirmwareRelease(ctx context.Context, id uint) (*FirmwareRelease, error)
	GetFirmwareByVersion(ctx context.Context, version string) (*FirmwareRelease, error)
	ListFirmwareReleases(ctx context.Context, channel string) ([]*FirmwareRelease, error)
	GetLatestApprovedFirmware(ctx context.Context, channel string) (*FirmwareRelease, error)

	// Firmware test operations
	CreateFirmwareTestResult(ctx context.Context, test *FirmwareTestResult) error
	UpdateFirmwareTestResult(ctx context.Context, test *FirmwareTestResult) error
	GetFirmwareTestResults(ctx context.Context, firmwareID uint) ([]*FirmwareTestResult, error)
	GetTestResultBySessionID(ctx context.Context, sessionID uint) (*FirmwareTestResult, error)

	// Update session operations
	CreateUpdateSession(ctx context.Context, session *UpdateSession) error
	UpdateUpdateSession(ctx context.Context, session *UpdateSession) error
	GetUpdateSession(ctx context.Context, sessionID string) (*UpdateSession, error)
	GetActiveDeviceUpdate(ctx context.Context, deviceID uint) (*UpdateSession, error)
	ListDeviceUpdateHistory(ctx context.Context, deviceID uint) ([]*UpdateSession, error)

	// Access token operations
	GetAccessToken(ctx context.Context, token string) (*AccessToken, error)
	CreateAccessToken(ctx context.Context, accessToken *AccessToken) error
	UpdateTokenLastAccess(ctx context.Context, token string) error
	DeleteAccessToken(ctx context.Context, token string) error

	// Transaction support
	WithTransaction(ctx context.Context, fn func(context.Context, DataStore) error) error
}

// dataStore implements DataStore interface
type dataStore struct {
	db *gorm.DB
}

// NewDataStore creates a new data store instance
func NewDataStore(db *gorm.DB) DataStore {
	return &dataStore{db: db}
}

// WithTransaction executes operations within a database transaction
func (ds *dataStore) WithTransaction(ctx context.Context, fn func(context.Context, DataStore) error) error {
	return ds.db.Transaction(func(tx *gorm.DB) error {
		txStore := NewDataStore(tx)
		return fn(ctx, txStore)
	})
}

// Device operations

func (ds *dataStore) CreateDevice(ctx context.Context, device *Device) error {
	return ds.db.WithContext(ctx).Create(device).Error
}

func (ds *dataStore) UpdateDevice(ctx context.Context, device *Device) error {
	return ds.db.WithContext(ctx).Save(device).Error
}

func (ds *dataStore) GetDevice(ctx context.Context, id uint) (*Device, error) {
	var device Device
	err := ds.db.WithContext(ctx).Preload("Organization").First(&device, id).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

func (ds *dataStore) GetDeviceByUID(ctx context.Context, uid string) (*Device, error) {
	var device Device
	err := ds.db.WithContext(ctx).Preload("Organization").
		Where("device_uid = ?", uid).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

func (ds *dataStore) ListDevicesByOrganization(ctx context.Context, orgID uint) ([]*Device, error) {
	var devices []*Device
	query := ds.db.WithContext(ctx).Preload("Organization")
	if orgID > 0 {
		query = query.Where("organization_id = ?", orgID)
	}
	err := query.Find(&devices).Error
	return devices, err
}

func (ds *dataStore) UpdateDeviceHeartbeat(ctx context.Context, deviceID uint) error {
	now := time.Now()
	return ds.db.WithContext(ctx).Model(&Device{}).
		Where("id = ?", deviceID).
		Update("last_heartbeat", now).Error
}

// Telemetry operations

func (ds *dataStore) SaveTelemetry(ctx context.Context, telemetry *Telemetry) error {
	return ds.db.WithContext(ctx).Create(telemetry).Error
}

func (ds *dataStore) SaveTelemetryBatch(ctx context.Context, telemetry []*Telemetry) error {
	if len(telemetry) == 0 {
		return nil
	}
	return ds.db.WithContext(ctx).CreateInBatches(telemetry, 100).Error
}

func (ds *dataStore) MarkTelemetryProcessed(ctx context.Context, messageID string) error {
	now := time.Now()
	return ds.db.WithContext(ctx).Model(&Telemetry{}).
		Where("message_id = ?", messageID).
		Updates(map[string]interface{}{
			"processed_at": now,
			"published_at": now,
		}).Error
}

func (ds *dataStore) GetDeviceTelemetry(ctx context.Context, deviceID uint, limit int) ([]*Telemetry, error) {
	var telemetry []*Telemetry
	query := ds.db.WithContext(ctx).Where("device_id = ?", deviceID).
		Order("received_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&telemetry).Error
	return telemetry, err
}

func (ds *dataStore) GetUnprocessedTelemetry(ctx context.Context, limit int) ([]*Telemetry, error) {
	var telemetry []*Telemetry
	query := ds.db.WithContext(ctx).
		Where("processed_at IS NULL").
		Order("received_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&telemetry).Error
	return telemetry, err
}

func (ds *dataStore) GetTelemetryByTimeRange(ctx context.Context, deviceID uint, start, end time.Time) ([]*Telemetry, error) {
	var telemetry []*Telemetry
	query := ds.db.WithContext(ctx).
		Where("device_id = ? AND received_at >= ? AND received_at <= ?", deviceID, start, end).
		Order("received_at ASC")
	err := query.Find(&telemetry).Error
	return telemetry, err
}

func (ds *dataStore) GetFailedTelemetry(ctx context.Context, limit int) ([]*Telemetry, error) {
	var telemetry []*Telemetry
	query := ds.db.WithContext(ctx).
		Where("last_error IS NOT NULL AND retry_count < ?", 5).
		Order("received_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&telemetry).Error
	return telemetry, err
}

func (ds *dataStore) UpdateTelemetryRetryCount(ctx context.Context, messageID string, retryCount int, lastError string) error {
	return ds.db.WithContext(ctx).Model(&Telemetry{}).
		Where("message_id = ?", messageID).
		Updates(map[string]interface{}{
			"retry_count": retryCount,
			"last_error":  lastError,
		}).Error
}

// Organization operations

func (ds *dataStore) CreateOrganization(ctx context.Context, org *Organization) error {
	return ds.db.WithContext(ctx).Create(org).Error
}

func (ds *dataStore) UpdateOrganization(ctx context.Context, org *Organization) error {
	return ds.db.WithContext(ctx).Save(org).Error
}

func (ds *dataStore) GetOrganization(ctx context.Context, id uint) (*Organization, error) {
	var org Organization
	err := ds.db.WithContext(ctx).First(&org, id).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func (ds *dataStore) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	var orgs []*Organization
	err := ds.db.WithContext(ctx).Find(&orgs).Error
	return orgs, err
}

// Firmware operations

func (ds *dataStore) CreateFirmwareRelease(ctx context.Context, firmware *FirmwareRelease) error {
	return ds.db.WithContext(ctx).Create(firmware).Error
}

func (ds *dataStore) UpdateFirmwareRelease(ctx context.Context, firmware *FirmwareRelease) error {
	return ds.db.WithContext(ctx).Save(firmware).Error
}

func (ds *dataStore) GetFirmwareRelease(ctx context.Context, id uint) (*FirmwareRelease, error) {
	var firmware FirmwareRelease
	err := ds.db.WithContext(ctx).Preload("TestResults").First(&firmware, id).Error
	if err != nil {
		return nil, err
	}
	return &firmware, nil
}

func (ds *dataStore) GetFirmwareByVersion(ctx context.Context, version string) (*FirmwareRelease, error) {
	var firmware FirmwareRelease
	err := ds.db.WithContext(ctx).Where("version = ?", version).First(&firmware).Error
	if err != nil {
		return nil, err
	}
	return &firmware, nil
}

func (ds *dataStore) ListFirmwareReleases(ctx context.Context, channel string) ([]*FirmwareRelease, error) {
	var releases []*FirmwareRelease
	query := ds.db.WithContext(ctx).Preload("TestResults")
	if channel != "" {
		query = query.Where("release_channel = ?", channel)
	}
	err := query.Order("created_at DESC").Find(&releases).Error
	return releases, err
}

func (ds *dataStore) GetLatestApprovedFirmware(ctx context.Context, channel string) (*FirmwareRelease, error) {
	var firmware FirmwareRelease
	query := ds.db.WithContext(ctx).Where("release_status = ?", ReleaseStatusApproved)
	if channel != "" {
		query = query.Where("release_channel = ?", channel)
	}
	err := query.Order("created_at DESC").First(&firmware).Error
	if err != nil {
		return nil, err
	}
	return &firmware, nil
}

// Firmware test operations

func (ds *dataStore) CreateFirmwareTestResult(ctx context.Context, test *FirmwareTestResult) error {
	return ds.db.WithContext(ctx).Create(test).Error
}

func (ds *dataStore) UpdateFirmwareTestResult(ctx context.Context, test *FirmwareTestResult) error {
	return ds.db.WithContext(ctx).Save(test).Error
}

func (ds *dataStore) GetFirmwareTestResults(ctx context.Context, firmwareID uint) ([]*FirmwareTestResult, error) {
	var tests []*FirmwareTestResult
	err := ds.db.WithContext(ctx).
		Where("firmware_id = ?", firmwareID).
		Order("created_at DESC").
		Find(&tests).Error
	return tests, err
}

func (ds *dataStore) GetTestResultBySessionID(ctx context.Context, sessionID uint) (*FirmwareTestResult, error) {
	var test FirmwareTestResult
	err := ds.db.WithContext(ctx).
		Where("update_session_id = ?", sessionID).
		First(&test).Error
	if err != nil {
		return nil, err
	}
	return &test, nil
}

// Update session operations

func (ds *dataStore) CreateUpdateSession(ctx context.Context, session *UpdateSession) error {
	return ds.db.WithContext(ctx).Create(session).Error
}

func (ds *dataStore) UpdateUpdateSession(ctx context.Context, session *UpdateSession) error {
	return ds.db.WithContext(ctx).Save(session).Error
}

func (ds *dataStore) GetUpdateSession(ctx context.Context, sessionID string) (*UpdateSession, error) {
	var session UpdateSession
	err := ds.db.WithContext(ctx).Preload("Device").Preload("Firmware").
		Where("session_id = ?", sessionID).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (ds *dataStore) GetActiveDeviceUpdate(ctx context.Context, deviceID uint) (*UpdateSession, error) {
	var session UpdateSession
	err := ds.db.WithContext(ctx).
		Where("device_id = ? AND update_status IN ?", deviceID,
			[]string{UpdateStatusInitiated, UpdateStatusDownloading}).
		Order("created_at DESC").First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (ds *dataStore) ListDeviceUpdateHistory(ctx context.Context, deviceID uint) ([]*UpdateSession, error) {
	var sessions []*UpdateSession
	err := ds.db.WithContext(ctx).Preload("Firmware").
		Where("device_id = ?", deviceID).
		Order("created_at DESC").Find(&sessions).Error
	return sessions, err
}

// Access token operations

func (ds *dataStore) GetAccessToken(ctx context.Context, token string) (*AccessToken, error) {
	var accessToken AccessToken
	err := ds.db.WithContext(ctx).Where("token = ?", token).First(&accessToken).Error
	if err != nil {
		return nil, err
	}
	return &accessToken, nil
}

func (ds *dataStore) CreateAccessToken(ctx context.Context, accessToken *AccessToken) error {
	return ds.db.WithContext(ctx).Create(accessToken).Error
}

func (ds *dataStore) UpdateTokenLastAccess(ctx context.Context, token string) error {
	now := time.Now()
	return ds.db.WithContext(ctx).Model(&AccessToken{}).
		Where("token = ?", token).
		Update("last_access_at", now).Error
}

func (ds *dataStore) DeleteAccessToken(ctx context.Context, token string) error {
	return ds.db.WithContext(ctx).Where("token = ?", token).Delete(&AccessToken{}).Error
}
