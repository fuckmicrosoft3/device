package models

import (
	"time"
	
	"gorm.io/gorm"
)

// Model is the base model with common fields for all database entities
type Model struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// AuthorizationLevel represents the level of access for an API key
type AuthorizationLevel int

const (
	// NoAuthLevel represents public access with no authentication
	NoAuthLevel AuthorizationLevel = 0
	// ViewerAuthLevel represents read-only access
	ViewerAuthLevel AuthorizationLevel = 1
	// WriterAuthLevel represents read-write access
	WriterAuthLevel AuthorizationLevel = 2
	// SudoAuthLevel represents administrative access
	SudoAuthLevel AuthorizationLevel = 3
	// RegisteredDeviceAuthLevel represents authentication for registered devices
	RegisteredDeviceAuthLevel AuthorizationLevel = 5
)

// APIKey represents an API token with associated access level
type APIKey struct {
	Model
	Key               string            `json:"key" gorm:"uniqueIndex;Column:key"`
	Name              string            `json:"name" gorm:"Column:name"`
	AuthorizationLevel AuthorizationLevel `json:"authorization_level" gorm:"Column:authorization_level"`
	ExpiresAt         *time.Time        `json:"expires_at" gorm:"Column:expires_at"`
	LastUsedAt        *time.Time        `json:"last_used_at" gorm:"Column:last_used_at"`
}

// ReleaseType is an enum for firmware release types
type ReleaseType string

const (
	// ReleaseTypeProduction represents a production release
	ReleaseTypeProduction ReleaseType = "production"
	// ReleaseTypeDevelopment represents a development release
	ReleaseTypeDevelopment ReleaseType = "development"
	// ReleaseTypeTest represents a test release
	ReleaseTypeTest ReleaseType = "test"
)

// Organization model represents a customer or department
type Organization struct {
	Model
	Name    string `json:"name" gorm:"Column:name"`
	URI     string `json:"uri" gorm:"Column:uri"`
	Active  bool   `json:"active" gorm:"Column:active"`
	Persist bool   `json:"persist" gorm:"Column:persist"`
}

// Device model represents a physical device in the system
type Device struct {
	Model
	UID             string           `json:"uid" gorm:"Column:uuid"`
	Serial          *string          `json:"serial" gorm:"Column:serial"`
	Organization    *Organization    `json:"organization" gorm:"foreignKey:OrganizationID"`
	OrganizationID  uint             `json:"organization_id" gorm:"Column:organization_id"`
	CurrentRelease  *FirmwareRelease `json:"release" gorm:"foreignKey:current_release_id"`
	CurrentReleaseID *uint           `json:"current_release_id" gorm:"Column:current_release_id"`
	Active          bool             `json:"active" gorm:"Column:active"`
	FilesPath       string           `json:"files_path" gorm:"Column:files_path"`
	AllowUpdates    bool             `json:"allow_updates" gorm:"Column:allow_updates"`
}

// DeviceMessage model represents messages received from devices
type DeviceMessage struct {
	UUID         string     `json:"uuid" gorm:"primary_key"`
	CreatedAt    time.Time  `json:"created_at" gorm:"Column:created_at"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"Column:updated_at"`
	Device       *Device    `json:"-" gorm:"foreignkey:DeviceID"`
	DeviceMCU    string     `json:"device_uid" gorm:"Column:device_mcu"`
	DeviceID     uint       `json:"device_id" gorm:"Column:device_id"`
	Message      string     `json:"message" gorm:"Column:message"`
	Published    bool       `json:"published" gorm:"Column:published"`
	PublishedAt  *time.Time `json:"published_at" gorm:"Column:published_at"`
	Republished  bool       `json:"republished" gorm:"Column:republished"`
	Error        bool       `json:"error" gorm:"Column:error"`
	ErrorMessage string     `json:"error_message" gorm:"Column:error_message"`
	SentVia      string     `json:"sent_via" gorm:"Column:sent_via"`
}

// FirmwareRelease model represents a basic firmware release
type FirmwareRelease struct {
	Model
	FilePath      string           `json:"file_path" gorm:"Column:file_path"`
	ReleaseType   ReleaseType      `json:"type" gorm:"Column:release_type"`
	Version       string           `json:"version" gorm:"Column:version"`
	Error         string           `json:"error" gorm:"Column:error"`
	Active        bool             `json:"active" gorm:"Column:active"`
	Size          uint             `json:"size"`
	Valid         bool             `json:"valid" gorm:"Column:valid"`
	IsTest        bool             `json:"is_test" gorm:"Column:is_test"`
	TestRelease   *FirmwareRelease `json:"test_release" gorm:"foreignKey:TestReleaseID"`
	TestReleaseID *uint            `json:"-" gorm:"Column:test_release_id"`
	TestDevice    *Device          `json:"-" gorm:"foreignkey:TestDeviceID"`
	TestDeviceID  *uint            `json:"test_device_id" gorm:"Column:test_device_id"`
	TestPassed    bool             `json:"test_passed" gorm:"Column:test_passed"`
	FileHash      string           `json:"file_hash" gorm:"-"`
}

// SemanticVersion represents a semantic version with major, minor, and patch components
type SemanticVersion struct {
	Major      uint `json:"major"`
	Minor      uint `json:"minor"`
	Patch      uint `json:"patch"`
	PreRelease string `json:"pre_release,omitempty"`
	Build      string `json:"build,omitempty"`
}

// FirmwareReleaseExtended enhances the FirmwareRelease model with additional fields
// for semantic versioning and signing. It embeds the base FirmwareRelease model
// to include all its fields, while adding extension fields for more advanced functionality.
type FirmwareReleaseExtended struct {
	FirmwareRelease            // Embed the base model
	MajorVersion      uint     `json:"major_version" gorm:"Column:major_version"`
	MinorVersion      uint     `json:"minor_version" gorm:"Column:minor_version"`
	PatchVersion      uint     `json:"patch_version" gorm:"Column:patch_version"`
	PreReleaseVersion string   `json:"pre_release_version" gorm:"Column:pre_release_version"`
	BuildMetadata     string   `json:"build_metadata" gorm:"Column:build_metadata"`
	Signature         string   `json:"signature" gorm:"Column:signature"`
	SignatureAlgorithm string  `json:"signature_algorithm" gorm:"Column:signature_algorithm;default:'ecdsa-secp256r1'"`
	SignedAt          *time.Time `json:"signed_at" gorm:"Column:signed_at"`
	SignedBy          string   `json:"signed_by" gorm:"Column:signed_by"`
	CertificateID     string   `json:"certificate_id" gorm:"Column:certificate_id"`
	RequiredVersion   string   `json:"required_version" gorm:"Column:required_version"`
	ReleaseNotes      string   `json:"release_notes" gorm:"Column:release_notes;type:text"`
	ChangesFrom       *uint    `json:"changes_from" gorm:"Column:changes_from"`
	ProductionRelease *FirmwareRelease `json:"production_release" gorm:"foreignKey:ProductionReleaseID"`
	ProductionReleaseID *uint  `json:"production_release_id" gorm:"Column:production_release_id"`
	DeltaPackages     bool     `json:"delta_packages" gorm:"Column:delta_packages"`
	RequiresFullImage bool     `json:"requires_full_image" gorm:"Column:requires_full_image"`
}

// FirmwareReleaseValidation represents validation results for a firmware release
type FirmwareReleaseValidation struct {
	Model
	FirmwareRelease   *FirmwareRelease `json:"firmware_release" gorm:"foreignKey:FirmwareReleaseID"`
	FirmwareReleaseID uint             `json:"firmware_release_id" gorm:"Column:firmware_release_id"`
	ValidationStatus  string           `json:"validation_status" gorm:"Column:validation_status"`
	ValidationErrors  string           `json:"validation_errors" gorm:"Column:validation_errors;type:text"`
	ValidatedAt       time.Time        `json:"validated_at" gorm:"Column:validated_at"`
	ValidatedBy       string           `json:"validated_by" gorm:"Column:validated_by"`
	SignatureValid    bool             `json:"signature_valid" gorm:"Column:signature_valid"`
	HashValid         bool             `json:"hash_valid" gorm:"Column:hash_valid"`
	SizeValid         bool             `json:"size_valid" gorm:"Column:size_valid"`
	VersionValid      bool             `json:"version_valid" gorm:"Column:version_valid"`
}

// FirmwareManifest provides metadata for available firmware
type FirmwareManifest struct {
	Model
	Releases          []FirmwareRelease `json:"releases" gorm:"many2many:manifest_releases"`
	ManifestVersion   string            `json:"manifest_version" gorm:"Column:manifest_version"`
	GeneratedAt       time.Time         `json:"generated_at" gorm:"Column:generated_at"`
	Signature         string            `json:"signature" gorm:"Column:signature"`
	SignatureAlgorithm string           `json:"signature_algorithm" gorm:"Column:signature_algorithm"`
	MinimumVersion    string            `json:"minimum_version" gorm:"Column:minimum_version"`
	RecommendedVersion string           `json:"recommended_version" gorm:"Column:recommended_version"`
}

// OTAUpdateStatus represents the status of an OTA update process
type OTAUpdateStatus string

const (
	// OTAStatusScheduled represents an update that is scheduled but not yet started
	OTAStatusScheduled OTAUpdateStatus = "scheduled"
	// OTAStatusPending represents an update that is ready to start
	OTAStatusPending OTAUpdateStatus = "pending"
	// OTAStatusAcknowledged represents an update that has been acknowledged by the device
	OTAStatusAcknowledged OTAUpdateStatus = "acknowledged"
	// OTAStatusDownloading represents an update that is currently being downloaded
	OTAStatusDownloading OTAUpdateStatus = "downloading"
	// OTAStatusDownloaded represents an update that has been downloaded completely
	OTAStatusDownloaded OTAUpdateStatus = "downloaded"
	// OTAStatusVerifying represents an update that is being verified
	OTAStatusVerifying OTAUpdateStatus = "verifying"
	// OTAStatusInstalling represents an update that is being installed
	OTAStatusInstalling OTAUpdateStatus = "installing"
	// OTAStatusCompleted represents an update that has been successfully completed
	OTAStatusCompleted OTAUpdateStatus = "completed"
	// OTAStatusFailed represents an update that has failed
	OTAStatusFailed OTAUpdateStatus = "failed"
	// OTAStatusRolledBack represents an update that was rolled back
	OTAStatusRolledBack OTAUpdateStatus = "rolled_back"
	// OTAStatusCancelled represents an update that was cancelled
	OTAStatusCancelled OTAUpdateStatus = "cancelled"
	// OTAStatusInProgress represents a batch with some completed and some in-progress updates
	OTAStatusInProgress OTAUpdateStatus = "in_progress"
)

// OTAUpdateSession represents a firmware update session for a device
type OTAUpdateSession struct {
	Model
	SessionID          string           `json:"session_id" gorm:"uniqueIndex;Column:session_id"`
	Device             *Device          `json:"device" gorm:"foreignKey:DeviceID"`
	DeviceID           uint             `json:"device_id" gorm:"Column:device_id"`
	FirmwareRelease    *FirmwareRelease `json:"firmware_release" gorm:"foreignKey:FirmwareReleaseID"`
	FirmwareReleaseID  uint             `json:"firmware_release_id" gorm:"Column:firmware_release_id"`
	Status             OTAUpdateStatus  `json:"status" gorm:"Column:status"`
	ScheduledAt        time.Time        `json:"scheduled_at" gorm:"Column:scheduled_at"`
	AcknowledgedAt     *time.Time       `json:"acknowledged_at" gorm:"Column:acknowledged_at"`
	DownloadStartedAt  *time.Time       `json:"download_started_at" gorm:"Column:download_started_at"`
	DownloadCompletedAt *time.Time      `json:"download_completed_at" gorm:"Column:download_completed_at"`
	VerificationStartedAt *time.Time    `json:"verification_started_at" gorm:"Column:verification_started_at"`
	VerificationCompletedAt *time.Time  `json:"verification_completed_at" gorm:"Column:verification_completed_at"`
	InstallStartedAt   *time.Time       `json:"install_started_at" gorm:"Column:install_started_at"`
	InstallCompletedAt *time.Time       `json:"install_completed_at" gorm:"Column:install_completed_at"`
	CompletedAt        *time.Time       `json:"completed_at" gorm:"Column:completed_at"`
	FailedAt           *time.Time       `json:"failed_at" gorm:"Column:failed_at"`
	ErrorMessage       string           `json:"error_message" gorm:"Column:error_message;type:text"`
	RetryCount         uint             `json:"retry_count" gorm:"Column:retry_count"`
	MaxRetries         uint             `json:"max_retries" gorm:"Column:max_retries;default:3"`
	BytesDownloaded    uint64           `json:"bytes_downloaded" gorm:"Column:bytes_downloaded"`
	TotalBytes         uint64           `json:"total_bytes" gorm:"Column:total_bytes"`
	DownloadSpeed      uint64           `json:"download_speed" gorm:"Column:download_speed"`
	LastChunkTime      *time.Time       `json:"last_chunk_time" gorm:"Column:last_chunk_time"`
	LastChunkSize      uint             `json:"last_chunk_size" gorm:"Column:last_chunk_size"`
	ChunksTotal        uint             `json:"chunks_total" gorm:"Column:chunks_total"`
	ChunksReceived     uint             `json:"chunks_received" gorm:"Column:chunks_received"`
	BatchID            string           `json:"batch_id" gorm:"Column:batch_id"`
	Priority           uint             `json:"priority" gorm:"Column:priority;default:5"`
	ForceUpdate        bool             `json:"force_update" gorm:"Column:force_update;default:false"`
	AllowRollback      bool             `json:"allow_rollback" gorm:"Column:allow_rollback;default:true"`
	DeviceVersion      string           `json:"device_version" gorm:"Column:device_version"`
	DownloadChecksum   string           `json:"download_checksum" gorm:"Column:download_checksum"`
	IsRollback         bool             `json:"is_rollback" gorm:"Column:is_rollback"`
	RollbackFromSession *uint           `json:"rollback_from_session" gorm:"Column:rollback_from_session"`
	PreviousVersion    string           `json:"previous_version" gorm:"Column:previous_version"`
	UpdateType         string           `json:"update_type" gorm:"Column:update_type;default:'full'"`
}

// OTAUpdateBatch represents a batch of updates for multiple devices
type OTAUpdateBatch struct {
	Model
	BatchID            string             `json:"batch_id" gorm:"uniqueIndex;Column:batch_id"`
	FirmwareRelease    *FirmwareRelease   `json:"firmware_release" gorm:"foreignKey:FirmwareReleaseID"`
	FirmwareReleaseID  uint               `json:"firmware_release_id" gorm:"Column:firmware_release_id"`
	Sessions           []OTAUpdateSession `json:"sessions" gorm:"foreignKey:BatchID;references:BatchID"`
	Status             OTAUpdateStatus    `json:"status" gorm:"Column:status"`
	ScheduledAt        time.Time          `json:"scheduled_at" gorm:"Column:scheduled_at"`
	Priority           uint               `json:"priority" gorm:"Column:priority;default:5"`
	ForceUpdate        bool               `json:"force_update" gorm:"Column:force_update;default:false"`
	AllowRollback      bool               `json:"allow_rollback" gorm:"Column:allow_rollback;default:true"`
	CompletedCount     uint               `json:"completed_count" gorm:"Column:completed_count"`
	FailedCount        uint               `json:"failed_count" gorm:"Column:failed_count"`
	PendingCount       uint               `json:"pending_count" gorm:"Column:pending_count"`
	TotalCount         uint               `json:"total_count" gorm:"Column:total_count"`
	CreatedBy          string             `json:"created_by" gorm:"Column:created_by"`
	CompletedAt        *time.Time         `json:"completed_at" gorm:"Column:completed_at"`
	Notes              string             `json:"notes" gorm:"Column:notes;type:text"`
	OrganizationID     *uint              `json:"organization_id" gorm:"Column:organization_id"`
	MaxConcurrent      uint               `json:"max_concurrent" gorm:"Column:max_concurrent;default:100"`
	UpdateType         string             `json:"update_type" gorm:"Column:update_type;default:'full'"`
}

// OTADeviceLog represents a log entry for OTA events on a device
type OTADeviceLog struct {
	Model
	Device             *Device            `json:"device" gorm:"foreignKey:DeviceID"`
	DeviceID           uint               `json:"device_id" gorm:"Column:device_id"`
	OTAUpdateSession   *OTAUpdateSession  `json:"ota_update_session" gorm:"foreignKey:SessionID;references:SessionID"`
	SessionID          string             `json:"session_id" gorm:"Column:session_id"`
	EventType          string             `json:"event_type" gorm:"Column:event_type"`
	LogLevel           string             `json:"log_level" gorm:"Column:log_level;default:'info'"`
	Message            string             `json:"message" gorm:"Column:message;type:text"`
	Metadata           string             `json:"metadata" gorm:"Column:metadata;type:text"`
	ErrorCode          *int               `json:"error_code" gorm:"Column:error_code"`
}



