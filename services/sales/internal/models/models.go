package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// Device represents a physical device
type Device struct {
	ID                     uuid.UUID               `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt              time.Time               `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt              time.Time               `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt              gorm.DeletedAt          `gorm:"index" json:"-"`
	MCU                    string                  `gorm:"column:mcu;not null;uniqueIndex" json:"mcu"`
	DeviceMachineRevisions []DeviceMachineRevision `gorm:"foreignKey:DeviceID" json:"-"`
}

// DeviceMachineRevision links a device to a machine revision
type DeviceMachineRevision struct {
	ID               uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt        time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt        gorm.DeletedAt    `gorm:"index" json:"-"`
	DeviceID         uuid.UUID         `gorm:"type:uuid;not null" json:"device_id"`
	Start            time.Time         `json:"start"`
	Termination      *time.Time        `json:"termination"`
	Active           bool              `gorm:"not null" json:"active"`
	Device           Device            `gorm:"foreignKey:DeviceID" json:"-"`
	MachineRevisions []MachineRevision `gorm:"foreignKey:DeviceMachineRevisionID" json:"-"`
}

// MachineRevision represents a revision of a machine
type MachineRevision struct {
	ID                      uuid.UUID             `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt               time.Time             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt               time.Time             `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt               gorm.DeletedAt        `gorm:"index" json:"-"`
	DeviceMachineRevisionID uuid.UUID             `gorm:"type:uuid;not null" json:"device_machine_revision_id"`
	MachineID               uuid.UUID             `gorm:"type:uuid;not null" json:"machine_id"`
	TemplateID              uuid.UUID             `gorm:"type:uuid;not null" json:"template_id"`
	TenantID                uuid.UUID             `gorm:"type:uuid;not null" json:"tenant_id"`
	Active                  bool                  `gorm:"not null" json:"active"`
	Start                   time.Time             `json:"start"`
	Terminate               *time.Time            `json:"terminate"`
	DeviceMachineRevision   DeviceMachineRevision `gorm:"foreignKey:DeviceMachineRevisionID" json:"-"`
	Machine                 Machine               `gorm:"foreignKey:MachineID" json:"-"`
	Template                Template              `gorm:"foreignKey:TemplateID" json:"-"`
	Tenant                  Tenant                `gorm:"foreignKey:TenantID" json:"-"`
	Sales                   []Sale                `gorm:"foreignKey:MachineRevisionID" json:"-"`
}

// Machine represents a physical machine
type Machine struct {
	ID               uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt        time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt        gorm.DeletedAt    `gorm:"index" json:"-"`
	MachineTypeID    uuid.UUID         `gorm:"type:uuid;not null" json:"machine_type_id"`
	MachineModelID   uuid.UUID         `gorm:"type:uuid;not null" json:"machine_model_id"`
	OrganizationID   *uuid.UUID        `gorm:"type:uuid" json:"organization_id"`
	UserID           *uuid.UUID        `gorm:"type:uuid" json:"user_id"`
	EditorID         *uuid.UUID        `gorm:"type:uuid" json:"editor_id"`
	TenantID         uuid.UUID         `gorm:"type:uuid;not null" json:"tenant_id"`
	Name             string            `gorm:"not null" json:"name"`
	ServiceTag       string            `gorm:"not null" json:"service_tag"`
	SerialTag        string            `gorm:"not null" json:"serial_tag"`
	IsActive         bool              `json:"is_active"`
	Attributes       []byte            `gorm:"type:jsonb" json:"attributes"`
	Account          *string           `json:"account"`
	Resource         *string           `json:"resource"`
	ResourceType     *string           `json:"resource_type"`
	ResourceDomain   *string           `json:"resource_domain"`
	Enabled          *bool             `json:"enabled"`
	Stats            []byte            `gorm:"type:json" json:"stats"`
	Configuration    []byte            `gorm:"type:json" json:"configuration"`
	MachineType      MachineType       `gorm:"foreignKey:MachineTypeID" json:"-"`
	MachineModel     MachineModel      `gorm:"foreignKey:MachineModelID" json:"-"`
	MachineRevisions []MachineRevision `gorm:"foreignKey:MachineID" json:"-"`
	Location         Location          `gorm:"foreignKey:MachineID" json:"-"`
}

// MachineType represents a type of machine
type MachineType struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `gorm:"not null" json:"name"`
	TenantID  uuid.UUID      `gorm:"type:uuid;not null" json:"tenant_id"`
	Tenant    Tenant         `gorm:"foreignKey:TenantID" json:"-"`
}

// MachineModel represents a model of machine
type MachineModel struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	OrganizationID *uuid.UUID     `gorm:"type:uuid" json:"organization_id"`
	UserID         *uuid.UUID     `gorm:"type:uuid" json:"user_id"`
	EditorID       *uuid.UUID     `gorm:"type:uuid" json:"editor_id"`
	TenantID       uuid.UUID      `gorm:"type:uuid;not null" json:"tenant_id"`
	Name           string         `gorm:"not null" json:"name"`
	Attributes     []byte         `gorm:"type:jsonb" json:"attributes"`
	Account        *string        `json:"account"`
	Resource       *string        `json:"resource"`
	ResourceType   *string        `json:"resource_type"`
	ResourceDomain *string        `json:"resource_domain"`
	Enabled        *bool          `json:"enabled"`
	Tenant         Tenant         `gorm:"foreignKey:TenantID" json:"-"`
}

// Template represents a machine template
type Template struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	MachineTypeID  *uuid.UUID     `gorm:"type:uuid" json:"machine_type_id"`
	MachineModelID *uuid.UUID     `gorm:"type:uuid" json:"machine_model_id"`
	UserID         *uuid.UUID     `gorm:"type:uuid" json:"user_id"`
	EditorID       *uuid.UUID     `gorm:"type:uuid" json:"editor_id"`
	OrganizationID *uuid.UUID     `gorm:"type:uuid" json:"organization_id"`
	TenantID       uuid.UUID      `gorm:"type:uuid;not null" json:"tenant_id"`
	Structure      []byte         `gorm:"type:json;not null" json:"structure"`
	Name           string         `gorm:"not null" json:"name"`
	IsDeployed     bool           `gorm:"not null" json:"is_deployed"`
	Attributes     []byte         `gorm:"type:jsonb" json:"attributes"`
	Account        *string        `json:"account"`
	Resource       *string        `json:"resource"`
	ResourceType   *string        `json:"resource_type"`
	ResourceDomain *string        `json:"resource_domain"`
	Enabled        *bool          `json:"enabled"`
	IsArchived     *bool          `json:"is_archived"`
	Tenant         Tenant         `gorm:"foreignKey:TenantID" json:"-"`
}

// Tenant represents a tenant
type Tenant struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Organization represents an organization
type Organization struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `gorm:"not null" json:"name"`
}

// Location represents a physical location
type Location struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	MachineID uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex" json:"machine_id"`
	Address   string         `gorm:"not null" json:"address"`
}

// Position represents a position in a machine
type Position struct {
	ID         int        `gorm:"primaryKey" json:"id"`
	ScaffoldID int        `gorm:"not null" json:"scaffold_id"`
	ProductID  *uuid.UUID `gorm:"type:uuid" json:"product_id"`
	MachineID  *uuid.UUID `gorm:"type:uuid" json:"machine_id"`
	TenantID   *uuid.UUID `gorm:"type:uuid" json:"tenant_id"`
	Position   int        `gorm:"not null" json:"position"`
}

// DispenseSession represents a dispense session
type DispenseSession struct {
	ID                            uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt                     time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                     time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt                     gorm.DeletedAt `gorm:"index" json:"-"`
	EventType                     string         `gorm:"not null" json:"event_type"`
	ExpectedDispense              float64        `gorm:"not null;default:0" json:"expected_dispense"`
	RemainingVolume               float64        `gorm:"not null;default:0" json:"remaining_volume"`
	ProductType                   int            `gorm:"not null;default:1" json:"product_type"`
	AmountKsh                     int32          `gorm:"not null" json:"amount_ksh"`
	DispenseState                 int            `gorm:"not null;default:0" json:"dispense_state"`
	TotalPumpRuntime              int64          `gorm:"not null;default:0" json:"total_pump_runtime"`
	IdempotencyKey                uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex" json:"idempotency_key"`
	InterpolatedEngineeringVolume float64        `gorm:"not null;default:0" json:"interpolated_engineering_volume"`
	IsProcessed                   bool           `gorm:"not null;default:false" json:"is_processed"`
	Time                          *int32         `json:"time"`
	DeviceMcu                     *string        `json:"device_mcu"`
	Sales                         []Sale         `gorm:"foreignKey:DispenseSessionID" json:"-"`
}

// Sale represents a sale
type Sale struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt         time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
	MachineRevisionID uuid.UUID      `gorm:"type:uuid;not null" json:"machine_revision_id"`
	MachineID         uuid.UUID      `gorm:"type:uuid;not null" json:"machine_id"`
	TransactionID     *uuid.UUID     `gorm:"type:uuid" json:"transaction_id"`
	ProductID         *uuid.UUID     `gorm:"type:uuid" json:"product_id"`
	OrganizationID    *uuid.UUID     `gorm:"type:uuid" json:"organization_id"`
	UserID            *uuid.UUID     `gorm:"type:uuid" json:"user_id"`
	EditorID          *uuid.UUID     `gorm:"type:uuid" json:"editor_id"`
	TenantID          uuid.UUID      `gorm:"type:uuid;not null" json:"tenant_id"`
	UUID              *string        `json:"uuid"`
	Quantity          int            `gorm:"not null;default:1" json:"quantity"`
	Amount            *int32         `json:"amount"`
	Type              string         `gorm:"not null" json:"type"`
	Position          int            `gorm:"not null;default:0" json:"position"`
	ExtRef            *string        `json:"ext_ref"`
	IsReconciled      bool           `gorm:"not null;default:false" json:"is_reconciled"`
	IsValid           bool           `gorm:"not null;default:true" json:"is_valid"`
	Time              *time.Time     `json:"time"`
	Account           *string        `json:"account"`
	Resource          *string        `json:"resource"`
	ResourceType      *string        `json:"resource_type"`
	ResourceDomain    *string        `json:"resource_domain"`
	DispenseSessionID uuid.UUID      `gorm:"type:uuid;not null" json:"dispense_session_id"`
}

// SalePayload represents a sale request payload
type SalePayload struct {
	Amount          int32     `json:"a"`
	AVol            int       `json:"a_vol"`
	DVol            int       `json:"d_vol"`
	Device          string    `json:"device"`
	Dt              int       `json:"dt"`
	EVol            float64   `json:"e_vol"`
	EventType       string    `json:"ev"`
	Ms              int       `json:"ms"`
	P               int       `json:"p"`
	RemainingVolume float64   `json:"r_vol"`
	S               int       `json:"s"`
	Time            int32     `json:"t"`
	Tag             string    `json:"tag"`
	IdempotencyKey  uuid.UUID `json:"u"`
}

// SetupModels configures GORM models and runs migrations
func SetupModels(db *gorm.DB) error {
	// Apply all migrations
	err := db.AutoMigrate(
		&Device{},
		&DeviceMachineRevision{},
		&MachineRevision{},
		&Machine{},
		&MachineType{},
		&MachineModel{},
		&Template{},
		&Tenant{},
		&Organization{},
		&Location{},
		&Position{},
		&DispenseSession{},
		&Sale{},
	)

	if err != nil {
		return errors.Wrap(err, "failed to run auto migrations")
	}

	return nil
}
