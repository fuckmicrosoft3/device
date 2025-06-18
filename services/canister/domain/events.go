package domain

import (
	"time"
)

// EventType constants
const (
	// Canister events
	CanisterCreated       = "V1_CANISTER_CREATED"
	CanisterUpdated       = "V1_CANISTER_UPDATED"
	CanisterEntry         = "V1_CANISTER_ENTRY"
	CanisterExit          = "V1_CANISTER_EXIT"
	CanisterCheck         = "V1_CANISTER_CHECK"
	CanisterDamage        = "V1_CANISTER_DAMAGE"
	CanisterOrgCheckIn    = "V1_CANISTER_ORGANIZATION_CHECK_IN"
	CanisterOrgCheckOut   = "V1_CANISTER_ORGANIZATION_CHECK_OUT"
	CanisterRestoreDamage = "V1_CANISTER_RESTORE_DAMAGE"
	CanisterRestoreTamper = "V1_CANISTER_RESTORE_TAMPER"
	CanisterRefillerEntry = "V1_CANISTER_REFILLER_ENTRY"
	CanisterRefillerExit  = "V1_CANISTER_REFILLER_EXIT"
	CanisterRefillSession = "V1_CANISTER_REFILL_SESSION"

	// Delivery events
	DeliveryNoteCreated = "V1_DELIVERY_NOTE_CREATED"
	DeliveryItemsAdded  = "V1_DELIVERY_ITEMS_ADDED"
	DeliveryItemRemoved = "V1_DELIVERY_ITEM_REMOVED"
)

// Event represents a domain event
type Event struct {
	ID            string      `json:"id"`
	AggregateID   string      `json:"aggregate_id"`
	AggregateType string      `json:"aggregate_type"`
	Type          string      `json:"type"`
	Version       int         `json:"version"`
	Timestamp     time.Time   `json:"timestamp"`
	Data          interface{} `json:"data"`
}

// Canister Events

// CanisterCreatedEvent represents a canister created event
type CanisterCreatedEvent struct {
	CanisterID     string `json:"canister_id"`
	Tag            string `json:"tag"`
	MCU            string `json:"mcu"`
	Model          string `json:"model"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	OrganisationID string `json:"organisation_id"`
	Attributes     []byte `json:"attributes"`
}

// CanisterUpdatedEvent represents a canister updated event
type CanisterUpdatedEvent struct {
	CanisterID     string `json:"canister_id"`
	Tag            string `json:"tag"`
	MCU            string `json:"mcu"`
	Model          string `json:"model"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	OrganisationID string `json:"organisation_id"`
	Attributes     []byte `json:"attributes"`
}

// CanisterEntryEvent represents a canister entry event
type CanisterEntryEvent struct {
	CanisterID    string    `json:"canister_id"`
	ExchangerUUID string    `json:"exchanger_uuid"`
	Ev            string    `json:"ev"`
	Serial        string    `json:"serial"`
	MCU           string    `json:"mcu"`
	DeviceID      string    `json:"device_id"`
	Source        string    `json:"source"`
	SourceID      string    `json:"source_id"`
	SourceTopic   string    `json:"source_topic"`
	Payload       string    `json:"payload"`
	Duplicate     bool      `json:"duplicate"`
	Time          time.Time `json:"time"`
	MovementID    string    `json:"movement_id"`
}

// CanisterExitEvent represents a canister exit event
type CanisterExitEvent struct {
	CanisterID    string    `json:"canister_id"`
	ExchangerUUID string    `json:"exchanger_uuid"`
	Ev            string    `json:"ev"`
	Serial        string    `json:"serial"`
	MCU           string    `json:"mcu"`
	DeviceID      string    `json:"device_id"`
	Source        string    `json:"source"`
	SourceID      string    `json:"source_id"`
	SourceTopic   string    `json:"source_topic"`
	Payload       string    `json:"payload"`
	Duplicate     bool      `json:"duplicate"`
	Time          time.Time `json:"time"`
	MovementID    string    `json:"movement_id"`
}

// CanisterCheckEvent represents a canister check event
type CanisterCheckEvent struct {
	CanisterID    string    `json:"canister_id"`
	ExchangerUUID string    `json:"exchanger_uuid"`
	Ev            string    `json:"ev"`
	Serial        string    `json:"serial"`
	MCU           string    `json:"mcu"`
	DeviceID      string    `json:"device_id"`
	Source        string    `json:"source"`
	SourceID      string    `json:"source_id"`
	SourceTopic   string    `json:"source_topic"`
	Payload       string    `json:"payload"`
	Duplicate     bool      `json:"duplicate"`
	Time          time.Time `json:"time"`
	TracingHeader string    `json:"tracing_header"`
}

// CanisterDamageEvent represents a canister damage event
type CanisterDamageEvent struct {
	CanisterID    string `json:"canister_id"`
	Time          string `json:"time"`
	ReportedBy    string `json:"reported_by"`
	ReportedByOrg string `json:"reported_by_org"`
	Reason        string `json:"reason"`
}

// CanisterOrgCheckInEvent represents a canister organization check-in event
type CanisterOrgCheckInEvent struct {
	CanisterID  string `json:"canister_id"`
	MCU         string `json:"mcu"`
	Time        string `json:"time"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	UserOrgID   string `json:"user_org_id"`
	UserOrgName string `json:"user_org_name"`
}

// CanisterOrgCheckOutEvent represents a canister organization check-out event
type CanisterOrgCheckOutEvent struct {
	CanisterID  string `json:"canister_id"`
	MCU         string `json:"mcu"`
	Time        string `json:"time"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	UserOrgID   string `json:"user_org_id"`
	UserOrgName string `json:"user_org_name"`
}

// CanisterRestoreDamageEvent represents a canister restore damage event
type CanisterRestoreDamageEvent struct {
	CanisterID    string `json:"canister_id"`
	Time          string `json:"time"`
	RestoredBy    string `json:"restored_by"`
	RestoredByOrg string `json:"restored_by_org"`
}

// CanisterRestoreTamperEvent represents a canister restore tamper event
type CanisterRestoreTamperEvent struct {
	CanisterID    string `json:"canister_id"`
	Time          string `json:"time"`
	RestoredBy    string `json:"restored_by"`
	RestoredByOrg string `json:"restored_by_org"`
}

// CanisterRefillerEntryEvent represents a canister refiller entry event
type CanisterRefillerEntryEvent struct {
	CanisterID    string `json:"canister_id"`
	ExchangerUUID string `json:"exchanger_uuid"`
	Serial        string `json:"serial"`
	MCU           string `json:"mcu"`
	DeviceID      string `json:"device_id"`
	Source        string `json:"source"`
	SourceID      string `json:"source_id"`
	SourceTopic   string `json:"source_topic"`
	Payload       string `json:"payload"`
	RefillerID    string `json:"refiller_id"`
	SessionID     string `json:"session_id"`
	Status        int32  `json:"status"`
	Time          int32  `json:"time"`
}

// CanisterRefillerExitEvent represents a canister refiller exit event
type CanisterRefillerExitEvent struct {
	CanisterID    string `json:"canister_id"`
	ExchangerUUID string `json:"exchanger_uuid"`
	Serial        string `json:"serial"`
	MCU           string `json:"mcu"`
	DeviceID      string `json:"device_id"`
	Source        string `json:"source"`
	SourceID      string `json:"source_id"`
	SourceTopic   string `json:"source_topic"`
	Payload       string `json:"payload"`
	RefillerID    string `json:"refiller_id"`
	SessionID     string `json:"session_id"`
	Status        int32  `json:"status"`
	Time          int32  `json:"time"`
}

// CanisterRefillSessionEvent represents a canister refill session event
type CanisterRefillSessionEvent struct {
	CanisterID    string  `json:"canister_id"`
	ExchangerUUID string  `json:"exchanger_uuid"`
	Serial        string  `json:"serial"`
	Ev            string  `json:"ev"`
	RefillerID    string  `json:"refiller_id"`
	DeviceID      string  `json:"device_id"`
	Source        string  `json:"source"`
	SourceID      string  `json:"source_id"`
	SourceTopic   string  `json:"source_topic"`
	Payload       string  `json:"payload"`
	StartTime     int64   `json:"start_time"`
	CurrentTime   int64   `json:"current_time"`
	TargetVolume  float64 `json:"target_volume"`
	ActualVolume  float64 `json:"actual_volume"`
	Status        int32   `json:"status"`
	SessionID     string  `json:"session_id"`
}

// Delivery Events

// DeliveryNoteCreatedEvent represents a delivery note created event
type DeliveryNoteCreatedEvent struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Metadata       []byte `json:"-"`
}

// DeliveryItemsAddedEvent represents delivery items added event
type DeliveryItemsAddedEvent struct {
	DeliveryItems []DeliveryItem `json:"delivery_items"`
	Metadata      []byte         `json:"-"`
}

// DeliveryItemRemovedEvent represents a delivery item removed event
type DeliveryItemRemovedEvent struct {
	ID       string `json:"id"`
	Metadata []byte `json:"-"`
}
