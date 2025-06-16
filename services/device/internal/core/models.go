package core

import (
	"time"

	"gorm.io/gorm"
)

// Model is a base struct with common fields for all database models.
type Model struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"      json:"-"` // Omit from JSON output
}

// Organization represents a customer or tenant.
type Organization struct {
	Model
	Name    string   `gorm:"uniqueIndex"               json:"name"`
	Active  bool     `gorm:"default:true"              json:"active"`
	Devices []Device `gorm:"foreignKey:OrganizationID" json:"-"` // Avoid circular JSON
}

// Device represents an IoT device.
type Device struct {
	Model
	UID             string        `gorm:"uniqueIndex"            json:"uid"`
	Serial          string        `json:"serial"`
	OrganizationID  uint          `json:"organization_id"`
	Organization    *Organization `json:"organization,omitempty"`
	FirmwareVersion string        `json:"firmware_version"`
	Active          bool          `gorm:"default:true"           json:"active"`
	AllowUpdates    bool          `gorm:"default:true"           json:"allow_updates"`
	LastSeen        *time.Time    `json:"last_seen"`
}

// DeviceMessage represents a message received from a device.
type DeviceMessage struct {
	UUID        string     `gorm:"primaryKey"    json:"uuid"`
	DeviceID    uint       `gorm:"index"         json:"device_id"`
	Device      *Device    `json:"-"` // Avoid circular JSON
	Message     string     `gorm:"type:text"     json:"message"`
	Published   bool       `gorm:"default:false" json:"published"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// FirmwareRelease represents a version of device firmware.
type FirmwareRelease struct {
	Model
	Version        string       `gorm:"uniqueIndex"           json:"version"`
	ReleaseType    string       `json:"release_type"` // e.g., production, beta
	FilePath       string       `json:"-"`            // Internal path, not exposed
	FileHash       string       `json:"file_hash"`
	FileSize       int64        `json:"file_size"`
	Signature      string       `json:"signature"`
	Active         bool         `gorm:"default:false"         json:"active"`
	Valid          bool         `gorm:"default:false"         json:"valid"`
	ReleaseNotes   string       `gorm:"type:text"             json:"release_notes"`
	MinimumVersion string       `json:"minimum_version"`
	OTASessions    []OTASession `gorm:"foreignKey:FirmwareID" json:"-"`
}

// OTASession represents an Over-The-Air update session for a device.
type OTASession struct {
	Model
	SessionID       string           `gorm:"uniqueIndex"        json:"session_id"`
	DeviceID        uint             `gorm:"index"              json:"device_id"`
	Device          *Device          `json:"device,omitempty"`
	FirmwareID      uint             `json:"firmware_id"`
	Firmware        *FirmwareRelease `json:"firmware,omitempty"`
	Status          string           `json:"status"` // e.g., pending, downloading, completed
	Progress        int              `gorm:"default:0"          json:"progress"`
	StartedAt       *time.Time       `json:"started_at"`
	CompletedAt     *time.Time       `json:"completed_at"`
	ErrorMessage    string           `json:"error_message"`
	RetryCount      int              `gorm:"default:0"          json:"retry_count"` // Restored
	BytesDownloaded int64            `json:"bytes_downloaded"`
	DownloadSpeed   int64            `json:"download_speed"` // Restored
}

// APIKey represents an API authentication key.
type APIKey struct {
	Model
	Key         string     `json:"-"` // Key is sensitive, don't expose
	Name        string     `json:"name"`
	Permissions []string   `gorm:"type:text[]"  json:"permissions"` // Restored
	ExpiresAt   *time.Time `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
}
