package models

import (
	"time"

	"gorm.io/gorm"
)

// DeliveryNote represents a delivery note in the database
type DeliveryNote struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	DeliveryNoteID string         `gorm:"uniqueIndex" json:"delivery_note_id"`
	AggregateID    string         `gorm:"uniqueIndex" json:"aggregate_id"`
	OrganizationID string         `gorm:"index" json:"organization_id"`
	Status         string         `json:"status"`
	Attributes     []byte         `json:"attributes"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// DeliveryItem represents a delivery item in the database
type DeliveryItem struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	ItemID         string         `gorm:"uniqueIndex" json:"item_id"`
	DeliveryNoteID string         `gorm:"index" json:"delivery_note_id"`
	CanisterID     string         `gorm:"index" json:"canister_id"`
	Delivered      bool           `json:"delivered"`
	Attributes     []byte         `json:"attributes"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}