// services/device/internal/core/models.go
package core

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Constants for business logic
const (
	// Release channels
	ReleaseChannelProduction = "production"
	ReleaseChannelBeta       = "beta"
	ReleaseChannelAlpha      = "alpha"

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

	// Firmware test statuses
	FirmwareTestPending = "pending"
	FirmwareTestRunning = "running"
	FirmwareTestPassed  = "passed"
	FirmwareTestFailed  = "failed"
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
	ErrTestDeviceNotFound  = BusinessError{"DEVICE_005", "test device not found or inactive"}

	// Firmware errors
	ErrFirmwareNotFound    = BusinessError{"FIRMWARE_001", "firmware release not found"}
	ErrFirmwareInvalid     = BusinessError{"FIRMWARE_002", "firmware validation failed"}
	ErrFirmwareNotActive   = BusinessError{"FIRMWARE_003", "firmware release is not active"}
	ErrVersionDowngrade    = BusinessError{"FIRMWARE_004", "firmware version downgrade not allowed"}
	ErrFirmwareNotTested   = BusinessError{"FIRMWARE_005", "firmware must pass testing before approval"}
	ErrFirmwareTestPending = BusinessError{"FIRMWARE_006", "firmware test already pending"}

	// OTA errors
	ErrUpdateInProgress  = BusinessError{"OTA_001", "update already in progress"}
	ErrNoUpdateAvailable = BusinessError{"OTA_002", "no update available"}
	ErrUpdateFailed      = BusinessError{"OTA_003", "update failed"}
	ErrChecksumMismatch  = BusinessError{"OTA_004", "firmware checksum verification failed"}

	// Organization errors
	ErrOrganizationNotFound = BusinessError{"ORG_001", "organization not found"}
	ErrOrganizationInactive = BusinessError{"ORG_002", "organization is inactive"}

	// Telemetry errors
	ErrInvalidTelemetryType = BusinessError{"TELEMETRY_001", "invalid telemetry type"}
	ErrQueueNotConfigured   = BusinessError{"TELEMETRY_002", "queue not configured for telemetry type"}
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
	Name             string        `gorm:"uniqueIndex;not null" json:"name"`
	Active           bool          `gorm:"default:true" json:"active"`
	DeviceLimit      int           `gorm:"default:1000" json:"device_limit"`
	ConnectionString string        `gorm:"type:text" json:"-"` // Azure Service Bus connection
	QueueMappings    QueueMappings `gorm:"type:jsonb" json:"queue_mappings"`
	Devices          []Device      `gorm:"foreignKey:OrganizationID" json:"-"`
}

// QueueMappings represents the mapping of telemetry types to queues
type QueueMappings struct {
	Queues []QueueMapping `json:"queues"`
}

// QueueMapping represents a single queue configuration
type QueueMapping struct {
	QueueName      string   `json:"queue_name"`
	TelemetryTypes []string `json:"telemetry_types"`
}

// Scan implements sql.Scanner interface for QueueMappings
func (qm *QueueMappings) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into QueueMappings", value)
	}
	return json.Unmarshal(bytes, qm)
}

// Value implements driver.Valuer interface for QueueMappings
func (qm QueueMappings) Value() (driver.Value, error) {
	return json.Marshal(qm)
}

// Device represents an IoT edge device
type Device struct {
	BaseModel
	DeviceUID       string        `gorm:"uniqueIndex;not null" json:"device_uid"`
	SerialNumber    string        `gorm:"index" json:"serial_number"`
	OrganizationID  uint          `gorm:"not null" json:"organization_id"`
	Organization    *Organization `json:"organization,omitempty"`
	FirmwareVersion string        `json:"firmware_version"`
	Active          bool          `gorm:"default:true" json:"active"`
	UpdatesEnabled  bool          `gorm:"default:true" json:"updates_enabled"`
	IsTestDevice    bool          `gorm:"default:false" json:"is_test_device"`
	LastHeartbeat   *time.Time    `json:"last_heartbeat"`
	Telemetry       []Telemetry   `gorm:"foreignKey:DeviceID" json:"-"`
}

// Telemetry represents device telemetry data
type Telemetry struct {
	MessageID     string          `gorm:"primaryKey" json:"message_id"`
	DeviceID      uint            `gorm:"index;not null" json:"device_id"`
	Device        *Device         `json:"-"`
	TelemetryType string          `gorm:"index;not null" json:"telemetry_type"` // Type for routing
	Payload       json.RawMessage `gorm:"type:jsonb" json:"payload"`            // JSONB for PostgreSQL
	ProcessedAt   *time.Time      `json:"processed_at"`
	PublishedAt   *time.Time      `json:"published_at"`
	ReceivedAt    time.Time       `json:"received_at" gorm:"default:CURRENT_TIMESTAMP"`
	QueueName     string          `json:"queue_name,omitempty"` // Track which queue it was sent to
	RetryCount    int             `gorm:"default:0" json:"retry_count"`
	LastError     string          `gorm:"type:text" json:"last_error,omitempty"`
}

// FirmwareRelease represents a deployable firmware version
type FirmwareRelease struct {
	BaseModel
	Version          string               `gorm:"uniqueIndex;not null" json:"version"`
	ReleaseChannel   string               `gorm:"index;not null" json:"release_channel"` // production, beta, alpha
	StoragePath      string               `json:"-"`                                     // Internal path
	Checksum         string               `gorm:"not null" json:"checksum"`
	SizeBytes        int64                `gorm:"not null" json:"size_bytes"`
	DigitalSignature string               `json:"digital_signature,omitempty"`
	ReleaseStatus    string               `gorm:"default:'draft'" json:"release_status"` // draft, testing, approved, deprecated
	ReleaseNotes     string               `gorm:"type:text" json:"release_notes"`
	MinimumVersion   string               `json:"minimum_version,omitempty"`
	TestResults      []FirmwareTestResult `gorm:"foreignKey:FirmwareID" json:"test_results,omitempty"`
	UpdateSessions   []UpdateSession      `gorm:"foreignKey:FirmwareID" json:"-"`
}

// FirmwareTestResult represents the result of testing firmware on a test device
type FirmwareTestResult struct {
	BaseModel
	FirmwareID      uint             `gorm:"not null" json:"firmware_id"`
	Firmware        *FirmwareRelease `json:"firmware,omitempty"`
	TestDeviceUID   string           `gorm:"not null" json:"test_device_uid"`
	TestStatus      string           `gorm:"not null" json:"test_status"` // pending, running, passed, failed
	StartedAt       *time.Time       `json:"started_at"`
	CompletedAt     *time.Time       `json:"completed_at"`
	TestDuration    int64            `json:"test_duration_seconds,omitempty"`
	ErrorMessage    string           `gorm:"type:text" json:"error_message,omitempty"`
	TestMetrics     json.RawMessage  `gorm:"type:jsonb" json:"test_metrics,omitempty"`
	UpdateSessionID *uint            `json:"update_session_id,omitempty"`
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
	IsTestUpdate     bool             `gorm:"default:false" json:"is_test_update"`
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

// TelemetryPayload represents the common structure of telemetry data
type TelemetryPayload struct {
	MessageID      string                 `json:"message_id"`
	TelemetryType  string                 `json:"telemetry_type"`
	Data           map[string]interface{} `json:"data"`
	Timestamp      int64                  `json:"timestamp"`
	SequenceNumber int64                  `json:"sequence_number,omitempty"`
}

// GetQueueForTelemetryType returns the queue name for a given telemetry type
func (qm *QueueMappings) GetQueueForTelemetryType(telemetryType string) (string, error) {
	for _, queue := range qm.Queues {
		for _, t := range queue.TelemetryTypes {
			if t == telemetryType {
				return queue.QueueName, nil
			}
		}
	}
	return "", fmt.Errorf("no queue configured for telemetry type: %s", telemetryType)
}
