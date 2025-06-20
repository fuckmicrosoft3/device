package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel contains common fields for all models
type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// BeforeCreate hook to set UUID
func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

// Tenant represents a tenant in the multi-tenant system
type Tenant struct {
	BaseModel
	Name string `gorm:"not null" json:"name"`
}

// Organization represents an organization
type Organization struct {
	BaseModel
	TenantID uuid.UUID `gorm:"type:uuid;not null" json:"tenant_id"`
	Name     string    `gorm:"not null" json:"name"`

	// Relationships
	Tenant *Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

// User represents a system user
type User struct {
	BaseModel
	Email          string     `gorm:"uniqueIndex;not null" json:"email"`
	Name           string     `json:"name"`
	TenantID       uuid.UUID  `gorm:"type:uuid;not null" json:"tenant_id"`
	OrganizationID *uuid.UUID `gorm:"type:uuid" json:"organization_id,omitempty"`

	// Relationships
	Tenant       *Tenant       `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

// Product represents a product that can be sold
type Product struct {
	BaseModel
	Name     string    `gorm:"not null" json:"name"`
	Code     string    `gorm:"uniqueIndex;not null" json:"code"`
	Price    float64   `json:"price"`
	TenantID uuid.UUID `gorm:"type:uuid;not null" json:"tenant_id"`

	// Relationships
	Tenant *Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

// Transaction represents a payment transaction
type Transaction struct {
	BaseModel
	TransactionID     string    `gorm:"uniqueIndex;not null" json:"transaction_id"`
	Amount            float64   `json:"amount"`
	Currency          string    `gorm:"default:'KSH'" json:"currency"`
	PaymentMethod     string    `json:"payment_method"`
	Status            string    `json:"status"`
	ExternalReference string    `json:"external_reference,omitempty"`
	TenantID          uuid.UUID `gorm:"type:uuid;not null" json:"tenant_id"`

	// Relationships
	Tenant *Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}
