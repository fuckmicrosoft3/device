package models

import (
	"time"

	"gorm.io/gorm"
)

// Event represents a domain event in the database
type Event struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	EventID       string         `gorm:"uniqueIndex" json:"event_id"`
	AggregateID   string         `gorm:"index" json:"aggregate_id"`
	AggregateType string         `json:"aggregate_type"`
	EventType     string         `json:"event_type"`
	Data          []byte         `json:"data"`
	Metadata      []byte         `json:"metadata"`
	Version       int            `json:"version"`
	Timestamp     time.Time      `json:"timestamp"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	Error         *string        `json:"error"`
	Processed     bool           `gorm:"index" json:"processed"`
}