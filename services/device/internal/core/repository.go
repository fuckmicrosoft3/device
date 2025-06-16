package core

import (
	"context"
	"time"

	// Removed the import to `infrastructure` to break the cycle.
	"gorm.io/gorm"
)

// Repository defines the interface for data access operations.
// The interface remains unchanged.
type Repository interface {
	// Device operations
	CreateDevice(ctx context.Context, device *Device) error
	UpdateDevice(ctx context.Context, device *Device) error
	GetDevice(ctx context.Context, id uint) (*Device, error)
	GetDeviceByUID(ctx context.Context, uid string) (*Device, error)
	ListDevices(ctx context.Context, orgID uint) ([]*Device, error)
	UpdateDeviceLastSeen(ctx context.Context, deviceID uint) error

	// Message operations
	CreateMessage(ctx context.Context, message *DeviceMessage) error
	CreateMessageBatch(ctx context.Context, messages []*DeviceMessage) error
	MarkMessagePublished(ctx context.Context, uuid string) error
	ListDeviceMessages(ctx context.Context, deviceID uint, limit int) ([]*DeviceMessage, error)

	// Organization operations
	CreateOrganization(ctx context.Context, org *Organization) error
	GetOrganization(ctx context.Context, id uint) (*Organization, error)
	ListOrganizations(ctx context.Context) ([]*Organization, error)

	// Firmware operations
	CreateFirmware(ctx context.Context, firmware *FirmwareRelease) error
	UpdateFirmware(ctx context.Context, firmware *FirmwareRelease) error
	GetFirmware(ctx context.Context, id uint) (*FirmwareRelease, error)
	GetFirmwareByVersion(ctx context.Context, version string) (*FirmwareRelease, error)
	ListFirmware(ctx context.Context, releaseType string) ([]*FirmwareRelease, error)
	GetLatestActiveFirmware(ctx context.Context, releaseType string) (*FirmwareRelease, error)

	// OTA operations
	CreateOTASession(ctx context.Context, session *OTASession) error
	UpdateOTASession(ctx context.Context, session *OTASession) error
	GetOTASession(ctx context.Context, sessionID string) (*OTASession, error)
	GetActiveOTASessionForDevice(ctx context.Context, deviceID uint) (*OTASession, error)
	ListDeviceOTASessions(ctx context.Context, deviceID uint) ([]*OTASession, error)
	GetActiveOTASessions(ctx context.Context) ([]*OTASession, error)

	// API Key operations
	GetAPIKey(ctx context.Context, key string) (*APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, key string) error

	// Transaction support
	WithTransaction(ctx context.Context, fn func(context.Context, Repository) error) error
}

// repository now depends on the generic *gorm.DB, not a concrete type from infrastructure.
type repository struct {
	db *gorm.DB
}

// NewRepository now accepts a *gorm.DB, inverting the dependency.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// WithTransaction has been updated to correctly wrap the new repository struct.
func (r *repository) WithTransaction(ctx context.Context, fn func(c context.Context, r Repository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Create a new repository instance for the transaction
		txRepo := NewRepository(tx)
		return fn(ctx, txRepo)
	})
}

// --- All other method implementations remain the same ---

func (r *repository) CreateDevice(ctx context.Context, d *Device) error {
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *repository) UpdateDevice(ctx context.Context, d *Device) error {
	return r.db.WithContext(ctx).Save(d).Error
}

func (r *repository) GetDevice(ctx context.Context, id uint) (*Device, error) {
	var d Device
	err := r.db.WithContext(ctx).Preload("Organization").First(&d, id).Error
	return &d, err
}

func (r *repository) GetDeviceByUID(ctx context.Context, uid string) (*Device, error) {
	var d Device
	err := r.db.WithContext(ctx).Preload("Organization").Where("uid = ?", uid).First(&d).Error
	return &d, err
}

func (r *repository) ListDevices(ctx context.Context, orgID uint) ([]*Device, error) {
	var devices []*Device
	q := r.db.WithContext(ctx).Preload("Organization")
	if orgID > 0 {
		q = q.Where("organization_id = ?", orgID)
	}
	return devices, q.Find(&devices).Error
}

func (r *repository) UpdateDeviceLastSeen(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&Device{}).Where("id = ?", id).Update("last_seen", time.Now()).Error
}

func (r *repository) CreateMessage(ctx context.Context, m *DeviceMessage) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *repository) CreateMessageBatch(ctx context.Context, messages []*DeviceMessage) error {
	if len(messages) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(messages, 100).Error
}

func (r *repository) MarkMessagePublished(ctx context.Context, uuid string) error {
	return r.db.WithContext(ctx).Model(&DeviceMessage{}).
		Where("uuid = ?", uuid).
		Updates(map[string]interface{}{"published": true, "published_at": time.Now()}).Error
}

func (r *repository) ListDeviceMessages(ctx context.Context, id uint, limit int) ([]*DeviceMessage, error) {
	var messages []*DeviceMessage
	q := r.db.WithContext(ctx).Where("device_id = ?", id).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	return messages, q.Find(&messages).Error
}

func (r *repository) CreateOrganization(ctx context.Context, o *Organization) error {
	return r.db.WithContext(ctx).Create(o).Error
}

func (r *repository) GetOrganization(ctx context.Context, id uint) (*Organization, error) {
	var o Organization
	return &o, r.db.WithContext(ctx).First(&o, id).Error
}

func (r *repository) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	var orgs []*Organization
	return orgs, r.db.WithContext(ctx).Find(&orgs).Error
}

func (r *repository) CreateFirmware(ctx context.Context, fw *FirmwareRelease) error {
	return r.db.WithContext(ctx).Create(fw).Error
}

func (r *repository) UpdateFirmware(ctx context.Context, fw *FirmwareRelease) error {
	return r.db.WithContext(ctx).Save(fw).Error
}

func (r *repository) GetFirmware(ctx context.Context, id uint) (*FirmwareRelease, error) {
	var fw FirmwareRelease
	return &fw, r.db.WithContext(ctx).First(&fw, id).Error
}

func (r *repository) GetFirmwareByVersion(ctx context.Context, version string) (*FirmwareRelease, error) {
	var fw FirmwareRelease
	return &fw, r.db.WithContext(ctx).Where("version = ?", version).First(&fw).Error
}

func (r *repository) ListFirmware(ctx context.Context, t string) ([]*FirmwareRelease, error) {
	var releases []*FirmwareRelease
	q := r.db.WithContext(ctx)
	if t != "" {
		q = q.Where("release_type = ?", t)
	}
	return releases, q.Order("created_at DESC").Find(&releases).Error
}

func (r *repository) GetLatestActiveFirmware(ctx context.Context, t string) (*FirmwareRelease, error) {
	var fw FirmwareRelease
	q := r.db.WithContext(ctx).Where("active = ? AND valid = ?", true, true)
	if t != "" {
		q = q.Where("release_type = ?", t)
	}
	return &fw, q.Order("created_at DESC").First(&fw).Error
}

func (r *repository) CreateOTASession(ctx context.Context, s *OTASession) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *repository) UpdateOTASession(ctx context.Context, s *OTASession) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *repository) GetOTASession(ctx context.Context, id string) (*OTASession, error) {
	var s OTASession
	err := r.db.WithContext(ctx).Preload("Device").Preload("Firmware").Where("session_id = ?", id).First(&s).Error
	return &s, err
}

func (r *repository) GetActiveOTASessionForDevice(ctx context.Context, id uint) (*OTASession, error) {
	var s OTASession
	err := r.db.WithContext(ctx).Where("device_id = ? AND status IN ?", id, []string{"pending", "downloading"}).Order("created_at DESC").First(&s).Error
	return &s, err
}

func (r *repository) ListDeviceOTASessions(ctx context.Context, deviceID uint) ([]*OTASession, error) {
	var s []*OTASession
	err := r.db.WithContext(ctx).Preload("Firmware").Where("device_id = ?", deviceID).Order("created_at DESC").Find(&s).Error
	return s, err
}

func (r *repository) GetActiveOTASessions(ctx context.Context) ([]*OTASession, error) {
	var s []*OTASession
	err := r.db.WithContext(ctx).Preload("Device").Preload("Firmware").Where("status IN ?", []string{"downloading", "installing"}).Find(&s).Error
	return s, err
}

func (r *repository) GetAPIKey(ctx context.Context, key string) (*APIKey, error) {
	var apiKey APIKey
	return &apiKey, r.db.WithContext(ctx).Where("key = ?", key).First(&apiKey).Error
}

func (r *repository) UpdateAPIKeyLastUsed(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Model(&APIKey{}).Where("key = ?", key).Update("last_used_at", time.Now()).Error
}
