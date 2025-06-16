package model

import (
	"time"
)

// Base model fields shared by all models
type Base struct {
	UUID      string    `json:"uuid" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeviceType defines the type of device
type DeviceType string

const (
	// TransportType represents a transport device (truck)
	TransportType DeviceType = "op-transport"
	// MachineType represents a machine device
	MachineType DeviceType = "op-machine"
)

// Device represents a physical device in the field
type Device struct {
	Base
	MCU  string     `json:"mcu" gorm:"column:mcu;uniqueIndex"`
	Type DeviceType `json:"type" gorm:"column:type"`
}

// OperationType defines the type of operation
type OperationType uint

const (
	// RefillOperationType represents a refill operation
	RefillOperationType OperationType = iota
	// MaintenanceOperationType represents a maintenance operation
	MaintenanceOperationType
)

// OperationStatus defines the status of an operation
type OperationStatus uint

const (
	// ScheduledOperationStatus represents a scheduled operation
	ScheduledOperationStatus OperationStatus = iota
	// AcknowledgedOperationStatus represents an acknowledged operation
	AcknowledgedOperationStatus
	// InProgressOperationStatus represents an in-progress operation
	InProgressOperationStatus
	// CancelledOperationStatus represents a cancelled operation
	CancelledOperationStatus
	// CompleteOperationStatus represents a completed operation
	CompleteOperationStatus
	// ErrorOperationStatus represents an operation with an error
	ErrorOperationStatus
)

// OperationUnit defines the unit of measurement for an operation
type OperationUnit uint

const (
	// OperationUnitLitres represents a measurement in litres
	OperationUnitLitres OperationUnit = iota
	// OperationUnitGrams represents a measurement in grams
	OperationUnitGrams
)

// OperationGroup represents a group of operations (typically a truck route)
type OperationGroup struct {
	Base
	ScheduledAt       *time.Time      `json:"scheduled_at"`
	TransportDeviceID string          `json:"transport_device_id" gorm:"column:transport_device_id;type:uuid"`
	TransportDevice   *Device         `json:"-" gorm:"foreignKey:TransportDeviceID"`
	Type              OperationType   `json:"type"`
	Status            OperationStatus `json:"status"`
	Operations        []Operation     `json:"operations" gorm:"foreignKey:OperationGroupID"`
}

// Operation represents an individual operation task
type Operation struct {
	Base
	TransportDeviceID  string          `json:"transport_device_id" gorm:"column:transport_device_id;type:uuid"`
	TransportDevice    *Device         `json:"-" gorm:"foreignKey:TransportDeviceID"`
	TransportDeviceMCU string          `json:"transport_device_mcu"`
	DeviceID           string          `json:"device_id" gorm:"column:device_id;type:uuid"`
	Device             *Device         `json:"-" gorm:"foreignKey:DeviceID"`
	DeviceMCU          string          `json:"device_mcu"`
	OperationGroupID   string          `json:"operation_group_id" gorm:"column:operation_group_id;type:uuid"`
	OperationGroup     *OperationGroup `json:"-" gorm:"foreignKey:OperationGroupID"`
	Type               OperationType   `json:"type"`
	Status             OperationStatus `json:"status"`
	State              string          `json:"state"`
	Amount             float64         `json:"amount"`
	Unit               OperationUnit   `json:"unit"`
}

// EventType defines the type of operation event
type EventType string

const (
	// OperationAcknowledgementEvent represents an acknowledgement event
	OperationAcknowledgementEvent EventType = "op-ack"
	// OperationStartEvent represents a start event
	OperationStartEvent EventType = "op-start"
	// OperationStatusEvent represents a status update event
	OperationStatusEvent EventType = "op-status"
	// OperationCompleteEvent represents a completion event
	OperationCompleteEvent EventType = "op-complete"
	// OperationErrorEvent represents an error event
	OperationErrorEvent EventType = "op-error"
	// OperationCancelEvent represents a cancellation event
	OperationCancelEvent EventType = "op-cancel"
)

// OperationSession represents an execution session of an operation
type OperationSession struct {
	Base
	StartedAt           *time.Time      `json:"started_at"`
	CompletedAt         *time.Time      `json:"completed_at"`
	Complete            bool            `json:"complete"`
	Error               bool            `json:"error"`
	OperationID         string          `json:"operation_id" gorm:"column:operation_id;type:uuid"`
	Operation           *Operation      `json:"operation" gorm:"foreignKey:OperationID"`
	OperationGroupID    string          `json:"operation_group_id" gorm:"column:operation_group_id;type:uuid"`
	OperationGroup      *OperationGroup `json:"operation_group" gorm:"foreignKey:OperationGroupID"`
	TargetVolumeIn      *float64        `json:"target_volume_in"`
	TargetVolumeOut     *float64        `json:"target_volume_out"`
	SessionVolumeIn     *float64        `json:"session_volume_in"`
	SessionVolumeOut    *float64        `json:"session_volume_out"`
	CumulativeVolumeIn  *float64        `json:"cumulative_volume_in"`
	CumulativeVolumeOut *float64        `json:"cumulative_volume_out"`
}

// OperationEvent represents an event related to an operation
type OperationEvent struct {
	Base
	DeviceID           string          `json:"device_id" gorm:"column:device_id;type:uuid"`
	Device             *Device         `json:"device" gorm:"foreignKey:DeviceID"`
	DeviceType         DeviceType      `json:"device_type"`
	OperationID        *string         `json:"operation_id" gorm:"column:operation_id;type:uuid"`
	Operation          *Operation      `json:"operation" gorm:"foreignKey:OperationID"`
	OperationGroupID   string          `json:"operation_group_id" gorm:"column:operation_group_id;type:uuid"`
	OperationGroup     *OperationGroup `json:"operation_group" gorm:"foreignKey:OperationGroupID"`
	OperationSessionID *string         `json:"operation_session_id" gorm:"column:operation_session_id;type:uuid"`
	EventType          EventType       `json:"event_type"`
	Details            []byte          `json:"details" gorm:"type:jsonb"`
}

// EventTypeFromString converts a string to an EventType
func EventTypeFromString(event string) EventType {
	switch event {
	case "op-ack":
		return OperationAcknowledgementEvent
	case "op-start":
		return OperationStartEvent
	case "op-status":
		return OperationStatusEvent
	case "op-complete":
		return OperationCompleteEvent
	case "op-error":
		return OperationErrorEvent
	case "op-cancel":
		return OperationCancelEvent
	default:
		return ""
	}
}

// StatusFromString converts a string to an OperationStatus
func StatusFromString(status string) OperationStatus {
	switch status {
	case "scheduled":
		return ScheduledOperationStatus
	case "acknowledged":
		return AcknowledgedOperationStatus
	case "in-progress":
		return InProgressOperationStatus
	case "cancelled":
		return CancelledOperationStatus
	case "completed":
		return CompleteOperationStatus
	case "error":
		return ErrorOperationStatus
	default:
		return ScheduledOperationStatus
	}
}

// TypeFromString converts a string to an OperationType
func TypeFromString(typeStr string) OperationType {
	switch typeStr {
	case "refill":
		return RefillOperationType
	case "maintenance":
		return MaintenanceOperationType
	default:
		return RefillOperationType
	}
}

// String returns a string representation of OperationStatus
func (s OperationStatus) String() string {
	statusMap := map[OperationStatus]string{
		ScheduledOperationStatus:    "scheduled",
		AcknowledgedOperationStatus: "acknowledged",
		InProgressOperationStatus:   "in-progress",
		CancelledOperationStatus:    "cancelled",
		CompleteOperationStatus:     "completed",
		ErrorOperationStatus:        "error",
	}
	
	if str, ok := statusMap[s]; ok {
		return str
	}
	return "unknown"
}

// String returns a string representation of OperationType
func (t OperationType) String() string {
	typeMap := map[OperationType]string{
		RefillOperationType:      "refill",
		MaintenanceOperationType: "maintenance",
	}
	
	if str, ok := typeMap[t]; ok {
		return str
	}
	return "unknown"
}

// String returns a string representation of OperationUnit
func (u OperationUnit) String() string {
	unitMap := map[OperationUnit]string{
		OperationUnitLitres: "litres",
		OperationUnitGrams:  "grams",
	}
	
	if str, ok := unitMap[u]; ok {
		return str
	}
	return "unknown"
}