// services/device/internal/core/models.go
package core

import (
	"encoding/json"
	"time"
	"fmt"
)

// Device represents a physical IoT device
type Device struct {
	ID               uint         `json:"id" gorm:"primaryKey"`
	DeviceUID        string       `json:"device_uid" gorm:"uniqueIndex;not null"`
	SerialNumber     string       `json:"serial_number" gorm:"uniqueIndex;not null"`
	OrganizationID   uint         `json:"organization_id" gorm:"index;not null"`
	OrganizationName string       `json:"organization_name" gorm:"not null"` // e.g., "staging.app.ingestor"
	FirmwareVersion  string       `json:"firmware_version"`
	HardwareVersion  string       `json:"hardware_version"`
	Active           bool         `json:"active" gorm:"default:true"`
	UpdatesEnabled   bool         `json:"updates_enabled" gorm:"default:true"`
	LastHeartbeat    *time.Time   `json:"last_heartbeat"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	Organization     Organization `gorm:"foreignKey:OrganizationID"`
}

// Telemetry represents device telemetry data with business context
type Telemetry struct {
	ID                     string          `json:"id" gorm:"primaryKey"` // Server-generated UUID
	DeviceID               uint            `json:"device_id" gorm:"index;not null"`
	DeviceUID              string          `json:"device_uid" gorm:"index;not null"`
	DeviceSerialNumber     string          `json:"device_serial_number" gorm:"not null"`
	TelemetryType          string          `json:"telemetry_type" gorm:"index;not null"` // From "ev" field
	Payload                json.RawMessage `json:"payload" gorm:"type:jsonb"`
	ReceivedAt             time.Time       `json:"received_at" gorm:"index;not null"`
	Published              bool            `json:"published" gorm:"default:false;index"`
	PublishedAt            *time.Time      `json:"published_at"`
	PublishedToQueue       string          `json:"published_to_queue"` // Which queue it was sent to
	ProcessingError        bool            `json:"processing_error" gorm:"default:false"`
	ProcessingErrorMessage string          `json:"processing_error_message"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
	Device                 Device          `gorm:"foreignKey:DeviceID"`
}

// Organization represents a customer organization
type Organization struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	Name                string    `json:"name" gorm:"uniqueIndex;not null"` // e.g., "staging.app.ingestor"
	DisplayName         string    `json:"display_name"`
	Environment         string    `json:"environment" gorm:"not null"` // "staging" or "production"
	ServiceBusNamespace string    `json:"service_bus_namespace"`       // For queue routing
	Active              bool      `json:"active" gorm:"default:true"`
	DeviceLimit         int       `json:"device_limit" gorm:"default:100"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// FirmwareRelease represents a firmware version release
type FirmwareRelease struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	Version          string    `json:"version" gorm:"uniqueIndex;not null"`
	ReleaseChannel   string    `json:"release_channel" gorm:"index;not null"`
	StoragePath      string    `json:"storage_path" gorm:"not null"`
	Checksum         string    `json:"checksum" gorm:"not null"`
	SizeBytes        int64     `json:"size_bytes" gorm:"not null"`
	DigitalSignature string    `json:"digital_signature"`
	ReleaseStatus    string    `json:"release_status" gorm:"index;not null"`
	ReleaseNotes     string    `json:"release_notes"`
	MinimumVersion   string    `json:"minimum_version"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// UpdateSession represents an OTA update session
type UpdateSession struct {
	ID               uint            `json:"id" gorm:"primaryKey"`
	SessionID        string          `json:"session_id" gorm:"uniqueIndex;not null"`
	DeviceID         uint            `json:"device_id" gorm:"index;not null"`
	FirmwareID       uint            `json:"firmware_id" gorm:"index;not null"`
	UpdateStatus     string          `json:"update_status" gorm:"index;not null"`
	ProgressPercent  int             `json:"progress_percent" gorm:"default:0"`
	BytesTransferred int64           `json:"bytes_transferred" gorm:"default:0"`
	TransferRate     int64           `json:"transfer_rate"`
	StartedAt        *time.Time      `json:"started_at"`
	CompletedAt      *time.Time      `json:"completed_at"`
	FailureReason    string          `json:"failure_reason"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	Device           Device          `gorm:"foreignKey:DeviceID"`
	Firmware         FirmwareRelease `gorm:"foreignKey:FirmwareID"`
}

// AccessToken represents API authentication tokens
type AccessToken struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	Token          string     `json:"token" gorm:"uniqueIndex;not null"`
	Description    string     `json:"description"`
	Scopes         []string   `json:"scopes" gorm:"type:text[]"`
	LastAccessedAt *time.Time `json:"last_accessed_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TableName overrides for GORM
func (Device) TableName() string          { return "devices" }
func (Telemetry) TableName() string       { return "telemetry" }
func (Organization) TableName() string    { return "organizations" }
func (FirmwareRelease) TableName() string { return "firmware_releases" }
func (UpdateSession) TableName() string   { return "update_sessions" }
func (AccessToken) TableName() string     { return "access_tokens" }

// Constants for business processes
const (
	// Release channels
	ReleaseChannelAlpha      = "alpha"
	ReleaseChannelBeta       = "beta"
	ReleaseChannelProduction = "production"

	// Release statuses
	ReleaseStatusDraft      = "draft"
	ReleaseStatusTesting    = "testing"
	ReleaseStatusApproved   = "approved"
	ReleaseStatusDeprecated = "deprecated"

	// Update statuses
	UpdateStatusInitiated   = "initiated"
	UpdateStatusDownloading = "downloading"
	UpdateStatusCompleted   = "completed"
	UpdateStatusFailed      = "failed"
	UpdateStatusCancelled   = "cancelled"

	// Environments
	EnvironmentStaging    = "staging"
	EnvironmentProduction = "production"

	// Common telemetry types (from "ev" field)
	TelemetryTypeCheck         = "check"
	TelemetryTypeLocation      = "location"
	TelemetryTypeGPS           = "gps"
	TelemetryTypeSensors       = "sensors"
	TelemetryTypeTransaction   = "prec"
	TelemetryTypeDrop          = "drop"
	TelemetryTypePay           = "pay"
	TelemetryTypeSubscribe     = "subscribe"
	TelemetryTypeRefill        = "refill"
	TelemetryTypeError         = "error"
	TelemetryTypeNetworkStatus = "net"
	TelemetryTypeMemoryStatus  = "mem"
	TelemetryTypeCanisterCheck = "can_refill"
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

// GetID returns the telemetry ID
func (t *Telemetry) GetID() string {
	return t.ID
}

// GetTelemetryType returns the telemetry type
func (t *Telemetry) GetTelemetryType() string {
	return t.TelemetryType
}

// Queue routing configuration
type QueueRoute struct {
	QueueName      string   `json:"queue_name"`
	TelemetryTypes []string `json:"telemetry_types"`
}

type OrganizationQueueConfig struct {
	OrganizationName string       `json:"organization_name"`
	Environment      string       `json:"environment"`
	ConnectionString string       `json:"connection_string"`
	QueueRoutes      []QueueRoute `json:"queue_routes"`
}
