package core

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Constants for business logic
const (
	// Release channels
	ReleaseChannelProduction = "production"
	ReleaseChannelStaging    = "staging"
	ReleaseChannelTest       = "test"

	// Release statuses
	ReleaseStatusDraft      = "draft"
	ReleaseStatusTesting    = "testing"
	ReleaseStatusApproved   = "approved"
	ReleaseStatusDeprecated = "deprecated"

	// Update statuses
	UpdateStatusInitiated   = "initiated"
	UpdateStatusDownloading = "downloading"
	UpdateStatusInstalling  = "installing"
	UpdateStatusCompleted   = "completed"
	UpdateStatusFailed      = "failed"
)

// Business-specific errors
type BusinessError struct {
	Code    string
	Message string
}

func (e BusinessError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

var (
	// Device errors
	ErrDeviceNotFound      = BusinessError{"DEVICE_001", "device not found"}
	ErrDeviceAlreadyExists = BusinessError{"DEVICE_002", "device already registered"}
	ErrDeviceInactive      = BusinessError{"DEVICE_003", "device is inactive"}
	ErrDeviceUpdatesDenied = BusinessError{"DEVICE_004", "device updates are disabled"}

	// Firmware errors
	ErrFirmwareNotFound  = BusinessError{"FIRMWARE_001", "firmware release not found"}
	ErrFirmwareInvalid   = BusinessError{"FIRMWARE_002", "firmware validation failed"}
	ErrFirmwareNotActive = BusinessError{"FIRMWARE_003", "firmware release is not active"}
	ErrVersionDowngrade  = BusinessError{"FIRMWARE_004", "firmware version downgrade not allowed"}

	// OTA errors
	ErrUpdateInProgress  = BusinessError{"OTA_001", "update already in progress"}
	ErrNoUpdateAvailable = BusinessError{"OTA_002", "no update available"}
	ErrUpdateFailed      = BusinessError{"OTA_003", "update failed"}
	ErrChecksumMismatch  = BusinessError{"OTA_004", "firmware checksum verification failed"}

	// Organization errors
	ErrOrganizationNotFound = BusinessError{"ORG_001", "organization not found"}
	ErrOrganizationInactive = BusinessError{"ORG_002", "organization is inactive"}
)

// BaseModel contains common fields for all entities
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Organization represents a device tenant
type Organization struct {
	BaseModel
	Name        string   `gorm:"uniqueIndex;not null" json:"name"`
	Active      bool     `gorm:"default:true" json:"active"`
	DeviceLimit int      `gorm:"default:1000" json:"device_limit"` //Consider impl regional sub-orgs
	Devices     []Device `gorm:"foreignKey:OrganizationID" json:"-"`
	URI         string   `gorm:"type:text" json:"uri"`
}

// Device represents a PCB attached to a physical device
type Device struct {
	BaseModel
	DeviceUID       string        `gorm:"uniqueIndex;not null" json:"device_uid"`
	SerialNumber    string        `gorm:"index" json:"serial_number"`
	OrganizationID  uint          `gorm:"not null" json:"organization_id"`
	Organization    *Organization `json:"organization,omitempty"`
	FirmwareVersion string        `json:"firmware_version"`
	Active          bool          `gorm:"default:true" json:"active"`
	UpdatesEnabled  bool          `gorm:"default:true" json:"updates_enabled"`
	LastHeartbeat   *time.Time    `json:"last_heartbeat"`
	Telemetry       []Telemetry   `gorm:"foreignKey:DeviceID" json:"-"`
}

// Telemetry represents device telemetry data
type Telemetry struct {
	MessageID   string     `gorm:"primaryKey" json:"message_id"`
	DeviceID    uint       `gorm:"index;not null" json:"device_id"`
	Device      *Device    `json:"-"`
	Payload     string     `gorm:"type:text" json:"payload"`
	ProcessedAt *time.Time `json:"processed_at"`
	PublishedAt *time.Time `json:"published_at"`
	ReceivedAt  time.Time  `json:"received_at" gorm:"default:CURRENT_TIMESTAMP"`
}

// FirmwareRelease represents a deployable firmware version
type FirmwareRelease struct {
	BaseModel
	Version          string          `gorm:"uniqueIndex;not null" json:"version"`
	ReleaseChannel   string          `gorm:"index;not null" json:"release_channel"` // major, minor, patch
	StoragePath      string          `json:"-"`                                     // Internal path
	Checksum         string          `gorm:"not null" json:"checksum"`
	SizeBytes        int64           `gorm:"not null" json:"size_bytes"`
	DigitalSignature string          `json:"digital_signature,omitempty"`
	ReleaseStatus    string          `gorm:"default:'draft'" json:"release_status"` // draft, testing, approved, deprecated
	ReleaseNotes     string          `gorm:"type:text" json:"release_notes"`
	MinimumVersion   string          `json:"minimum_version,omitempty"`
	UpdateSessions   []UpdateSession `gorm:"foreignKey:FirmwareID" json:"-"`
}

// UpdateSession represents an OTA update process
type UpdateSession struct {
	BaseModel
	SessionID        string           `gorm:"uniqueIndex;not null" json:"session_id"`
	DeviceID         uint             `gorm:"index;not null" json:"device_id"`
	Device           *Device          `json:"device,omitempty"`
	FirmwareID       uint             `gorm:"not null" json:"firmware_id"`
	Firmware         *FirmwareRelease `json:"firmware,omitempty"`
	UpdateStatus     string           `gorm:"index;not null" json:"update_status"` // initiated, downloading, installing, completed, failed
	ProgressPercent  int              `gorm:"default:0" json:"progress_percent"`
	StartedAt        *time.Time       `json:"started_at"`
	CompletedAt      *time.Time       `json:"completed_at"`
	FailureReason    string           `json:"failure_reason,omitempty"`
	RetryAttempt     int              `gorm:"default:0" json:"retry_attempt"`
	BytesTransferred int64            `json:"bytes_transferred"`
	TransferRate     int64            `json:"transfer_rate_bps"` // bytes per second
}

// AccessToken represents API authentication
type AccessToken struct {
	BaseModel
	Token        string     `gorm:"uniqueIndex;not null" json:"-"`
	Description  string     `json:"description"`
	Scopes       []string   `gorm:"type:text[]" json:"scopes"`
	ExpiresAt    *time.Time `json:"expires_at"`
	LastAccessAt *time.Time `json:"last_access_at"`
}
