package core

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// DataStore defines the interface for data persistence operations.
type DataStore interface {
	// Transaction support
	WithTransaction(ctx context.Context, fn func(ctx context.Context, tx DataStore) error) error

	// Device operations
	CreateDevice(ctx context.Context, device *Device) error
	GetDevice(ctx context.Context, id uint) (*Device, error)
	GetDeviceByUID(ctx context.Context, uid string) (*Device, error)
	UpdateDevice(ctx context.Context, device *Device) error
	UpdateDeviceHeartbeat(ctx context.Context, deviceID uint) error
	ListDevicesByOrganization(ctx context.Context, orgID uint) ([]*Device, error)

	// Telemetry operations
	SaveTelemetry(ctx context.Context, telemetry *Telemetry) error
	GetDeviceTelemetry(ctx context.Context, deviceID uint, limit int) ([]*Telemetry, error)
	UpdateTelemetryPublishStatus(ctx context.Context, telemetryID string, published bool, publishedAt *time.Time, queue string) error
	GetUnpublishedTelemetry(ctx context.Context, limit int) ([]*Telemetry, error)

	// Organization operations
	CreateOrganization(ctx context.Context, org *Organization) error
	GetOrganization(ctx context.Context, id uint) (*Organization, error)
	GetOrganizationByName(ctx context.Context, name string) (*Organization, error)
	UpdateOrganization(ctx context.Context, org *Organization) error
	ListOrganizations(ctx context.Context) ([]*Organization, error)

	// Firmware operations
	CreateFirmwareRelease(ctx context.Context, firmware *FirmwareRelease) error
	GetFirmwareRelease(ctx context.Context, id uint) (*FirmwareRelease, error)
	GetFirmwareByVersion(ctx context.Context, version string) (*FirmwareRelease, error)
	UpdateFirmwareRelease(ctx context.Context, firmware *FirmwareRelease) error
	ListFirmwareReleases(ctx context.Context, channel string) ([]*FirmwareRelease, error)
	GetLatestApprovedFirmware(ctx context.Context, channel string) (*FirmwareRelease, error)

	// Update session operations
	CreateUpdateSession(ctx context.Context, session *UpdateSession) error
	GetUpdateSession(ctx context.Context, sessionID string) (*UpdateSession, error)
	UpdateUpdateSession(ctx context.Context, session *UpdateSession) error
	GetActiveDeviceUpdate(ctx context.Context, deviceID uint) (*UpdateSession, error)

	// Access token operations
	CreateAccessToken(ctx context.Context, token *AccessToken) error
	GetAccessToken(ctx context.Context, token string) (*AccessToken, error)
	UpdateTokenLastAccess(ctx context.Context, token string) error
	DeleteAccessToken(ctx context.Context, token string) error
}

// dataStore is the concrete implementation of DataStore.
type dataStore struct {
	db *gorm.DB
}

// NewDataStore creates a new data store instance.
func NewDataStore(db *gorm.DB) DataStore {
	return &dataStore{db: db}
}

// WithTransaction executes a function within a database transaction.
func (s *dataStore) WithTransaction(ctx context.Context, fn func(ctx context.Context, tx DataStore) error) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		txStore := &dataStore{db: tx}
		return fn(ctx, txStore)
	})
}

// --- Device Operations ---

func (s *dataStore) CreateDevice(ctx context.Context, device *Device) error {
	return s.db.WithContext(ctx).Create(device).Error
}

func (s *dataStore) GetDevice(ctx context.Context, id uint) (*Device, error) {
	var device Device
	err := s.db.WithContext(ctx).Preload("Organization").First(&device, id).Error
	return &device, err
}

func (s *dataStore) GetDeviceByUID(ctx context.Context, uid string) (*Device, error) {
	var device Device
	err := s.db.WithContext(ctx).Preload("Organization").Where("device_uid = ?", uid).First(&device).Error
	return &device, err
}

func (s *dataStore) UpdateDevice(ctx context.Context, device *Device) error {
	return s.db.WithContext(ctx).Save(device).Error
}

func (s *dataStore) UpdateDeviceHeartbeat(ctx context.Context, deviceID uint) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&Device{}).Where("id = ?", deviceID).
		Update("last_heartbeat", now).Error
}

func (s *dataStore) ListDevicesByOrganization(ctx context.Context, orgID uint) ([]*Device, error) {
	var devices []*Device
	err := s.db.WithContext(ctx).Where("organization_id = ?", orgID).Find(&devices).Error
	return devices, err
}

// --- Telemetry Operations ---

func (s *dataStore) SaveTelemetry(ctx context.Context, telemetry *Telemetry) error {
	return s.db.WithContext(ctx).Create(telemetry).Error
}

func (s *dataStore) GetDeviceTelemetry(ctx context.Context, deviceID uint, limit int) ([]*Telemetry, error) {
	var telemetry []*Telemetry
	err := s.db.WithContext(ctx).
		Where("device_id = ?", deviceID).
		Order("received_at DESC").
		Limit(limit).
		Find(&telemetry).Error
	return telemetry, err
}

func (s *dataStore) UpdateTelemetryPublishStatus(ctx context.Context, telemetryID string, published bool, publishedAt *time.Time, queue string) error {
	updates := map[string]interface{}{
		"published":          published,
		"published_at":       publishedAt,
		"published_to_queue": queue,
		"updated_at":         time.Now(),
	}
	return s.db.WithContext(ctx).Model(&Telemetry{}).
		Where("id = ?", telemetryID).
		Updates(updates).Error
}

func (s *dataStore) GetUnpublishedTelemetry(ctx context.Context, limit int) ([]*Telemetry, error) {
	var telemetry []*Telemetry
	err := s.db.WithContext(ctx).
		Where("published = ?", false).
		Where("processing_error = ?", false).
		Order("received_at ASC").
		Limit(limit).
		Find(&telemetry).Error
	return telemetry, err
}

// --- Organization Operations ---

func (s *dataStore) CreateOrganization(ctx context.Context, org *Organization) error {
	return s.db.WithContext(ctx).Create(org).Error
}

func (s *dataStore) GetOrganization(ctx context.Context, id uint) (*Organization, error) {
	var org Organization
	err := s.db.WithContext(ctx).First(&org, id).Error
	return &org, err
}

func (s *dataStore) GetOrganizationByName(ctx context.Context, name string) (*Organization, error) {
	var org Organization
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&org).Error
	return &org, err
}

func (s *dataStore) UpdateOrganization(ctx context.Context, org *Organization) error {
	return s.db.WithContext(ctx).Save(org).Error
}

func (s *dataStore) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	var orgs []*Organization
	err := s.db.WithContext(ctx).Find(&orgs).Error
	return orgs, err
}

// --- Firmware Operations ---

func (s *dataStore) CreateFirmwareRelease(ctx context.Context, firmware *FirmwareRelease) error {
	return s.db.WithContext(ctx).Create(firmware).Error
}

func (s *dataStore) GetFirmwareRelease(ctx context.Context, id uint) (*FirmwareRelease, error) {
	var firmware FirmwareRelease
	err := s.db.WithContext(ctx).First(&firmware, id).Error
	return &firmware, err
}

func (s *dataStore) GetFirmwareByVersion(ctx context.Context, version string) (*FirmwareRelease, error) {
	var firmware FirmwareRelease
	err := s.db.WithContext(ctx).Where("version = ?", version).First(&firmware).Error
	return &firmware, err
}

func (s *dataStore) UpdateFirmwareRelease(ctx context.Context, firmware *FirmwareRelease) error {
	return s.db.WithContext(ctx).Save(firmware).Error
}

func (s *dataStore) ListFirmwareReleases(ctx context.Context, channel string) ([]*FirmwareRelease, error) {
	var releases []*FirmwareRelease
	query := s.db.WithContext(ctx)
	if channel != "" {
		query = query.Where("release_channel = ?", channel)
	}
	err := query.Order("created_at DESC").Find(&releases).Error
	return releases, err
}

func (s *dataStore) GetLatestApprovedFirmware(ctx context.Context, channel string) (*FirmwareRelease, error) {
	var firmware FirmwareRelease
	err := s.db.WithContext(ctx).
		Where("release_channel = ? AND release_status = ?", channel, ReleaseStatusApproved).
		Order("created_at DESC").
		First(&firmware).Error
	return &firmware, err
}

// --- Update Session Operations ---

func (s *dataStore) CreateUpdateSession(ctx context.Context, session *UpdateSession) error {
	return s.db.WithContext(ctx).Create(session).Error
}

func (s *dataStore) GetUpdateSession(ctx context.Context, sessionID string) (*UpdateSession, error) {
	var session UpdateSession
	err := s.db.WithContext(ctx).
		Preload("Device").
		Preload("Firmware").
		Where("session_id = ?", sessionID).
		First(&session).Error
	return &session, err
}

func (s *dataStore) UpdateUpdateSession(ctx context.Context, session *UpdateSession) error {
	return s.db.WithContext(ctx).Save(session).Error
}

func (s *dataStore) GetActiveDeviceUpdate(ctx context.Context, deviceID uint) (*UpdateSession, error) {
	var session UpdateSession
	err := s.db.WithContext(ctx).
		Where("device_id = ? AND update_status IN ?", deviceID,
			[]string{UpdateStatusInitiated, UpdateStatusDownloading}).
		First(&session).Error
	return &session, err
}

// --- Access Token Operations ---

func (s *dataStore) CreateAccessToken(ctx context.Context, token *AccessToken) error {
	return s.db.WithContext(ctx).Create(token).Error
}

func (s *dataStore) GetAccessToken(ctx context.Context, token string) (*AccessToken, error) {
	var accessToken AccessToken
	err := s.db.WithContext(ctx).Where("token = ?", token).First(&accessToken).Error
	return &accessToken, err
}

func (s *dataStore) UpdateTokenLastAccess(ctx context.Context, token string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&AccessToken{}).
		Where("token = ?", token).
		Update("last_accessed_at", now).Error
}

func (s *dataStore) DeleteAccessToken(ctx context.Context, token string) error {
	return s.db.WithContext(ctx).Where("token = ?", token).Delete(&AccessToken{}).Error
}
