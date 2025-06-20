package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// JSONB represents a JSONB database type
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(map[string]interface{})
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), j)
	}
	return json.Unmarshal(bytes, j)
}

// Device represents a physical device
type Device struct {
	ID  uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	MCU string    `gorm:"not null;index" json:"mcu"`
}

// TableName overrides the table name for Device
func (Device) TableName() string {
	return "devices"
}

// DeviceMachineRevision represents a device-machine revision mapping
type DeviceMachineRevision struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	DeviceID    uuid.UUID  `gorm:"type:uuid;not null" json:"device_id"`
	Active      bool       `gorm:"default:true" json:"active"`
	Start       *time.Time `json:"start"`
	Termination *time.Time `json:"termination"`

	// Relationships
	Device           *Device           `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	MachineRevisions []MachineRevision `gorm:"foreignKey:DeviceMachineRevisionID" json:"machine_revisions,omitempty"`
}

// TableName overrides the table name
func (DeviceMachineRevision) TableName() string {
	return "device_machine_revisions"
}

// MachineType represents a type of machine
type MachineType struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Name     string    `gorm:"not null" json:"name"`
	TenantID uuid.UUID `gorm:"type:uuid;not null" json:"tenant_id"`
}

// TableName overrides the table name
func (MachineType) TableName() string {
	return "machine_type"
}

// MachineModel represents a machine model
type MachineModel struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	Name           string     `gorm:"not null" json:"name"`
	TenantID       uuid.UUID  `gorm:"type:uuid;not null" json:"tenant_id"`
	OrganizationID *uuid.UUID `gorm:"type:uuid" json:"organization_id"`
	UserID         *uuid.UUID `gorm:"type:uuid" json:"user_id"`
	EditorID       *uuid.UUID `gorm:"type:uuid" json:"editor_id"`
	Attributes     JSONB      `gorm:"type:jsonb" json:"attributes"`
	Account        *string    `json:"account"`
	Resource       *string    `json:"resource"`
	ResourceType   *string    `json:"resource_type"`
	ResourceDomain *string    `json:"resource_domain"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Enabled        *bool      `json:"enabled"`
}

// TableName overrides the table name
func (MachineModel) TableName() string {
	return "machine_model"
}

// Template represents a machine template
type Template struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	Name           string     `gorm:"not null" json:"name"`
	Structure      JSONB      `gorm:"type:jsonb;not null" json:"structure"`
	MachineTypeID  *uuid.UUID `gorm:"type:uuid" json:"machine_type_id"`
	MachineModelID *uuid.UUID `gorm:"type:uuid" json:"machine_model_id"`
	TenantID       *uuid.UUID `gorm:"type:uuid" json:"tenant_id"`
	OrganizationID *uuid.UUID `gorm:"type:uuid" json:"organization_id"`
	UserID         *uuid.UUID `gorm:"type:uuid" json:"user_id"`
	EditorID       *uuid.UUID `gorm:"type:uuid" json:"editor_id"`
	IsDeployed     bool       `gorm:"not null" json:"is_deployed"`
	IsArchived     *bool      `json:"is_archived"`
	Attributes     JSONB      `gorm:"type:jsonb" json:"attributes"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TableName overrides the table name
func (Template) TableName() string {
	return "template"
}

// Machine represents a physical machine
type Machine struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	Name           string     `gorm:"not null" json:"name"`
	ServiceTag     string     `gorm:"not null" json:"service_tag"`
	SerialTag      string     `gorm:"not null" json:"serial_tag"`
	MachineTypeID  uuid.UUID  `gorm:"type:uuid;not null" json:"machine_type_id"`
	MachineModelID uuid.UUID  `gorm:"type:uuid;not null" json:"machine_model_id"`
	TenantID       *uuid.UUID `gorm:"type:uuid" json:"tenant_id"`
	OrganizationID *uuid.UUID `gorm:"type:uuid" json:"organization_id"`
	UserID         *uuid.UUID `gorm:"type:uuid" json:"user_id"`
	EditorID       *uuid.UUID `gorm:"type:uuid" json:"editor_id"`
	IsActive       *bool      `json:"is_active"`
	Enabled        *bool      `json:"enabled"`
	Attributes     JSONB      `gorm:"type:jsonb" json:"attributes"`
	Stats          JSONB      `gorm:"type:jsonb" json:"stats"`
	Configuration  JSONB      `gorm:"type:jsonb" json:"configuration"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Relationships
	MachineType  *MachineType  `gorm:"foreignKey:MachineTypeID" json:"machine_type,omitempty"`
	MachineModel *MachineModel `gorm:"foreignKey:MachineModelID" json:"machine_model,omitempty"`
}

// TableName overrides the table name
func (Machine) TableName() string {
	return "machines"
}

// MachineRevision represents a revision of a machine configuration
type MachineRevision struct {
	ID                      uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	DeviceMachineRevisionID uuid.UUID  `gorm:"type:uuid;not null" json:"device_machine_revision_id"`
	Active                  bool       `gorm:"not null" json:"active"`
	MachineID               uuid.UUID  `gorm:"type:uuid;not null" json:"machine_id"`
	TemplateID              uuid.UUID  `gorm:"type:uuid;not null" json:"template_id"`
	TenantID                uuid.UUID  `gorm:"type:uuid;not null" json:"tenant_id"`
	Start                   *time.Time `json:"start"`
	Terminate               *time.Time `json:"terminate"`

	// Relationships
	DeviceMachineRevision *DeviceMachineRevision `gorm:"foreignKey:DeviceMachineRevisionID" json:"device_machine_revision,omitempty"`
	Machine               *Machine               `gorm:"foreignKey:MachineID" json:"machine,omitempty"`
	Template              *Template              `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
}

// TableName overrides the table name
func (MachineRevision) TableName() string {
	return "machine_revisions"
}

// Position represents a product position in a machine
type Position struct {
	ID         int        `gorm:"primary_key" json:"id"`
	ScaffoldID int        `gorm:"not null" json:"scaffold_id"`
	ProductID  *uuid.UUID `gorm:"type:uuid" json:"product_id"`
	MachineID  *uuid.UUID `gorm:"type:uuid" json:"machine_id"`
	TenantID   *uuid.UUID `gorm:"type:uuid" json:"tenant_id"`
	Position   int        `gorm:"not null" json:"position"`
}

// TableName overrides the table name
func (Position) TableName() string {
	return "positions"
}

// Location represents a machine location
type Location struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	MachineID uuid.UUID `gorm:"type:uuid;not null;index" json:"machine_id"`
	Address   string    `gorm:"not null" json:"address"`

	// Relationships
	Machine *Machine `gorm:"foreignKey:MachineID" json:"machine,omitempty"`
}

// TableName overrides the table name
func (Location) TableName() string {
	return "locations"
}
