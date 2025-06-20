package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DispenseState represents the state of a dispense session
type DispenseState int

const (
	DispenseStatePending DispenseState = iota
	DispenseStateProcessing
	DispenseStateCompleted
	DispenseStateFailed
)

// SaleType represents the type of sale
type SaleType string

const (
	SaleTypePaidVend SaleType = "PAID_VEND"
	SaleTypeFreeVend SaleType = "FREE_VEND"
	SaleTypeRFIDVend SaleType = "RFID_VEND"
)

// EnrichmentStatus represents the enrichment status
type EnrichmentStatus string

const (
	EnrichmentStatusPending   EnrichmentStatus = "PENDING"
	EnrichmentStatusCompleted EnrichmentStatus = "COMPLETED"
	EnrichmentStatusFailed    EnrichmentStatus = "FAILED"
)

// DispenseSession represents a raw dispense event
type DispenseSession struct {
	ID                            uuid.UUID     `gorm:"type:uuid;primary_key" json:"id"`
	EventType                     string        `gorm:"not null" json:"event_type"`
	ExpectedDispense              float64       `gorm:"type:decimal(10,2);not null" json:"expected_dispense"`
	RemainingVolume               float64       `gorm:"type:decimal(10,2);not null" json:"remaining_volume"`
	ProductType                   int           `gorm:"default:1;not null" json:"product_type"`
	AmountKSH                     int32         `gorm:"not null" json:"amount_ksh"`
	DispenseState                 DispenseState `gorm:"default:0;not null" json:"dispense_state"`
	TotalPumpRuntime              int64         `gorm:"default:0;not null" json:"total_pump_runtime"`
	IdempotencyKey                uuid.UUID     `gorm:"type:uuid;uniqueIndex;not null" json:"idempotency_key"`
	InterpolatedEngineeringVolume float64       `gorm:"type:decimal(10,2);not null;default:0" json:"interpolated_engineering_volume"`
	IsProcessed                   bool          `gorm:"default:false;not null" json:"is_processed"`
	Time                          *int32        `json:"time"`
	DeviceMCU                     *string       `gorm:"index" json:"device_mcu"`
	CreatedAt                     time.Time     `json:"created_at"`
	UpdatedAt                     time.Time     `json:"updated_at"`

	// Enrichment tracking
	EnrichmentStatus   EnrichmentStatus `gorm:"default:'PENDING';not null" json:"enrichment_status"`
	EnrichmentAttempts int              `gorm:"default:0;not null" json:"enrichment_attempts"`
	LastEnrichmentAt   *time.Time       `json:"last_enrichment_at"`
	EnrichmentError    *string          `json:"enrichment_error"`
}

// BeforeCreate hook
func (d *DispenseSession) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

// GetSaleTime returns the sale time as a time.Time
func (d *DispenseSession) GetSaleTime() time.Time {
	if d.Time == nil {
		return d.CreatedAt
	}
	return time.Unix(int64(*d.Time), 0)
}

// Sale represents a processed and enriched sale record
type Sale struct {
	BaseModel

	// Core fields
	DispenseSessionID uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"dispense_session_id"`
	UUID              string    `json:"uuid,omitempty"`
	Quantity          int       `gorm:"not null" json:"quantity"`
	Amount            int32     `json:"amount"`
	Type              SaleType  `gorm:"type:varchar(16);not null" json:"type"`
	Position          int       `gorm:"not null;default:0" json:"position"`
	ExtRef            *string   `json:"ext_ref,omitempty"`
	IsReconciled      bool      `gorm:"default:false;not null" json:"is_reconciled"`
	IsValid           bool      `gorm:"default:true;not null" json:"is_valid"`
	Time              time.Time `json:"time"`

	// Foreign keys
	MachineRevisionID uuid.UUID  `gorm:"type:uuid;not null" json:"machine_revision_id"`
	MachineID         uuid.UUID  `gorm:"type:uuid;not null" json:"machine_id"`
	TransactionID     *uuid.UUID `gorm:"type:uuid" json:"transaction_id,omitempty"`
	ProductID         *uuid.UUID `gorm:"type:uuid" json:"product_id,omitempty"`
	OrganizationID    *uuid.UUID `gorm:"type:uuid" json:"organization_id,omitempty"`
	UserID            *uuid.UUID `gorm:"type:uuid" json:"user_id,omitempty"`
	EditorID          *uuid.UUID `gorm:"type:uuid" json:"editor_id,omitempty"`
	TenantID          uuid.UUID  `gorm:"type:uuid;not null" json:"tenant_id"`

	// Resource tracking
	Account        *string `json:"account,omitempty"`
	Resource       *string `json:"resource,omitempty"`
	ResourceType   *string `json:"resource_type,omitempty"`
	ResourceDomain *string `json:"resource_domain,omitempty"`

	// Relationships
	DispenseSession *DispenseSession `gorm:"foreignKey:DispenseSessionID" json:"dispense_session,omitempty"`
	MachineRevision *MachineRevision `gorm:"foreignKey:MachineRevisionID" json:"machine_revision,omitempty"`
	Machine         *Machine         `gorm:"foreignKey:MachineID" json:"machine,omitempty"`
	Transaction     *Transaction     `gorm:"foreignKey:TransactionID" json:"transaction,omitempty"`
	Product         *Product         `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Organization    *Organization    `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	User            *User            `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Editor          *User            `gorm:"foreignKey:EditorID" json:"editor,omitempty"`
	Tenant          *Tenant          `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

// DetermineSaleType determines the sale type based on amount and RFID
func DetermineSaleType(amount *int32, rfid *uint32) SaleType {
	if rfid != nil && *rfid == 1 {
		return SaleTypeRFIDVend
	}

	if amount == nil || *amount == 0 {
		return SaleTypeFreeVend
	}

	return SaleTypePaidVend
}

// SaleEnrichmentData represents the enriched data from ERP
type SaleEnrichmentData struct {
	Device          *Device
	MachineRevision *MachineRevision
	Machine         *Machine
	MachineType     *MachineType
	Location        *Location
	Position        *Position
	Tenant          *Tenant
}
