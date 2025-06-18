// services/device/internal/core/models.go
package core

import (
	"encoding/json"
	"time"
)

// Device represents a physical IoT device.
type Device struct {
	ID               uint         `gorm:"primaryKey"                json:"id"`
	DeviceUID        string       `gorm:"uniqueIndex;not null"      json:"device_uid"`
	SerialNumber     string       `gorm:"uniqueIndex;not null"      json:"serial_number"`
	OrganizationID   uint         `gorm:"index;not null"            json:"organization_id"`
	OrganizationName string       `gorm:"not null"                  json:"organization_name"` // e.g., "staging.app.ingestor"
	FirmwareVersion  string       `json:"firmware_version"`
	HardwareVersion  string       `json:"hardware_version"`
	Active           bool         `gorm:"default:true"              json:"active"`
	UpdatesEnabled   bool         `gorm:"default:true"              json:"updates_enabled"`
	LastHeartbeat    *time.Time   `json:"last_heartbeat"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	Organization     Organization `gorm:"foreignKey:OrganizationID"`
}

// Telemetry represents device telemetry data with business context.
type Telemetry struct {
	ID                     string          `gorm:"primaryKey"               json:"id"` // Server-generated UUID
	DeviceID               uint            `gorm:"index;not null"           json:"device_id"`
	DeviceUID              string          `gorm:"index;not null"           json:"device_uid"`
	DeviceSerialNumber     string          `gorm:"not null"                 json:"device_serial_number"`
	TelemetryType          string          `gorm:"index;not null"           json:"telemetry_type"` // From "ev" field
	Payload                json.RawMessage `gorm:"type:jsonb"               json:"payload"`
	ReceivedAt             time.Time       `gorm:"index;not null"           json:"received_at"`
	Published              bool            `gorm:"default:false;index"      json:"published"`
	PublishedAt            *time.Time      `json:"published_at"`
	PublishedToQueue       string          `json:"published_to_queue"` // Which queue it was sent to
	ProcessingError        bool            `gorm:"default:false"            json:"processing_error"`
	ProcessingErrorMessage string          `json:"processing_error_message"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
	Device                 Device          `gorm:"foreignKey:DeviceID"`
}

// Organization represents a customer organization.
type Organization struct {
	ID                  uint      `gorm:"primaryKey"            json:"id"`
	Name                string    `gorm:"uniqueIndex;not null"  json:"name"` // e.g., "staging.app.ingestor"
	DisplayName         string    `json:"display_name"`
	Environment         string    `gorm:"not null"              json:"environment"` // "staging" or "production"
	ServiceBusNamespace string    `json:"service_bus_namespace"`                    // For queue routing
	Active              bool      `gorm:"default:true"          json:"active"`
	DeviceLimit         int       `gorm:"default:100"           json:"device_limit"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// FirmwareRelease represents a firmware version release.
type FirmwareRelease struct {
	ID               uint      `gorm:"primaryKey"           json:"id"`
	Version          string    `gorm:"uniqueIndex;not null" json:"version"`
	ReleaseChannel   string    `gorm:"index;not null"       json:"release_channel"`
	StoragePath      string    `gorm:"not null"             json:"storage_path"`
	Checksum         string    `gorm:"not null"             json:"checksum"`
	SizeBytes        int64     `gorm:"not null"             json:"size_bytes"`
	DigitalSignature string    `json:"digital_signature"`
	ReleaseStatus    string    `gorm:"index;not null"       json:"release_status"`
	ReleaseNotes     string    `json:"release_notes"`
	MinimumVersion   string    `json:"minimum_version"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// UpdateSession represents an OTA update session.
type UpdateSession struct {
	ID               uint            `gorm:"primaryKey"            json:"id"`
	SessionID        string          `gorm:"uniqueIndex;not null"  json:"session_id"`
	DeviceID         uint            `gorm:"index;not null"        json:"device_id"`
	FirmwareID       uint            `gorm:"index;not null"        json:"firmware_id"`
	UpdateStatus     string          `gorm:"index;not null"        json:"update_status"`
	ProgressPercent  int             `gorm:"default:0"             json:"progress_percent"`
	BytesTransferred int64           `gorm:"default:0"             json:"bytes_transferred"`
	TransferRate     int64           `json:"transfer_rate"`
	StartedAt        *time.Time      `json:"started_at"`
	CompletedAt      *time.Time      `json:"completed_at"`
	FailureReason    string          `json:"failure_reason"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	Device           Device          `gorm:"foreignKey:DeviceID"`
	Firmware         FirmwareRelease `gorm:"foreignKey:FirmwareID"`
}

// AccessToken represents API authentication tokens.
type AccessToken struct {
	ID             uint       `gorm:"primaryKey"           json:"id"`
	Token          string     `gorm:"uniqueIndex;not null" json:"token"`
	Description    string     `json:"description"`
	Scopes         []string   `gorm:"type:text[]"          json:"scopes"`
	LastAccessedAt *time.Time `json:"last_accessed_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// UpdateBatch represents a batch OTA update job targeting multiple devices.
type UpdateBatch struct {
	ID         uint                `gorm:"primaryKey"               json:"id"`
	Name       string              `gorm:"not null"                 json:"name"`
	FirmwareID uint                `gorm:"index;not null"           json:"firmware_id"`
	Status     string              `gorm:"index;not null"           json:"status"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
	Firmware   FirmwareRelease     `gorm:"foreignKey:FirmwareID"`
	Devices    []UpdateBatchDevice `gorm:"foreignKey:UpdateBatchID"`
}

// UpdateBatchDevice represents the status of an update for a specific device within a batch.
type UpdateBatchDevice struct {
	ID              uint           `gorm:"primaryKey"                                      json:"id"`
	UpdateBatchID   uint           `gorm:"index;not null"                                  json:"update_batch_id"`
	DeviceID        uint           `gorm:"index;not null"                                  json:"device_id"`
	Status          string         `gorm:"index;not null"                                  json:"status"`
	UpdateSessionID *string        `gorm:"index"                                           json:"update_session_id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	UpdateBatch     UpdateBatch    `gorm:"foreignKey:UpdateBatchID"`
	Device          Device         `gorm:"foreignKey:DeviceID"`
	UpdateSession   *UpdateSession `gorm:"foreignKey:UpdateSessionID;references:SessionID"`
}

// TableName overrides for GORM.
func (Device) TableName() string            { return "devices" }
func (Telemetry) TableName() string         { return "telemetry" }
func (Organization) TableName() string      { return "organizations" }
func (FirmwareRelease) TableName() string   { return "firmware_releases" }
func (UpdateSession) TableName() string     { return "update_sessions" }
func (AccessToken) TableName() string       { return "access_tokens" }
func (UpdateBatch) TableName() string       { return "update_batches" }
func (UpdateBatchDevice) TableName() string { return "update_batch_devices" }

// Constants for business processes.
const (
	// Release channels.
	ReleaseChannelAlpha      = "alpha"
	ReleaseChannelBeta       = "beta"
	ReleaseChannelProduction = "production"

	// Release statuses.
	ReleaseStatusDraft      = "draft"
	ReleaseStatusTesting    = "testing"
	ReleaseStatusApproved   = "approved"
	ReleaseStatusDeprecated = "deprecated"

	// Update statuses.
	UpdateStatusInitiated    = "initiated"
	UpdateStatusAcknowledged = "acknowledged"
	UpdateStatusDownloading  = "downloading"
	UpdateStatusFlashed      = "flashed"
	UpdateStatusCompleted    = "completed"
	UpdateStatusFailed       = "failed"
	UpdateStatusCancelled    = "cancelled"

	// Environments.
	EnvironmentStaging    = "staging"
	EnvironmentProduction = "production"

	// Batch update statuses.
	BatchStatusPending    = "pending"
	BatchStatusInProgress = "in_progress"
	BatchStatusCompleted  = "completed"
	BatchStatusFailed     = "failed"

	// Batch device statuses.
	BatchDeviceStatusPending   = "pending"
	BatchDeviceStatusInitiated = "initiated"
	BatchDeviceStatusSucceeded = "succeeded"
	BatchDeviceStatusFailed    = "failed"

	// Common telemetry types (from "ev" field).
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

// GetID returns the telemetry ID.
func (t *Telemetry) GetID() string {
	return t.ID
}

// GetTelemetryType returns the telemetry type.
func (t *Telemetry) GetTelemetryType() string {
	return t.TelemetryType
}

// FirmwareMetadata contains metadata for firmware releases.
type FirmwareMetadata struct {
	Filename       string
	Version        string
	ReleaseChannel string
	ReleaseNotes   string
}

// DeviceRegistrationResult represents the result of a single device registration attempt.
type DeviceRegistrationResult struct {
	Device *Device `json:"device,omitempty"`
	Error  string  `json:"error,omitempty"`
}

// BatchRegistrationRequest represents a batch device registration request.
type BatchRegistrationRequest struct {
	DeviceUID       string `binding:"required"      json:"device_uid"`
	OrganizationID  uint   `binding:"required"      json:"organization_id"`
	SerialNumber    string `json:"serial_number"`
	HardwareVersion string `json:"hardware_version"`
}

// BatchRegistrationResponse represents the response from batch device registration.
type BatchRegistrationResponse struct {
	Successful []DeviceRegistrationResult `json:"successful"`
	Failed     []DeviceRegistrationResult `json:"failed"`
}

// CreateUpdateBatchRequest represents a request to create a batch update.
type CreateUpdateBatchRequest struct {
	Name       string `binding:"required" json:"name"`
	FirmwareID uint   `binding:"required" json:"firmware_id"`
	DeviceIDs  []uint `binding:"required" json:"device_ids"`
}
