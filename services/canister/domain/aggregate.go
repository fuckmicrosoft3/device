package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AggregateBase provides common aggregate functionality
type AggregateBase struct {
	id           string
	aggregateType string
	version      int
	events       []Event
	applier      func(event interface{}) error
}

// Aggregate is the interface for all aggregates
type Aggregate interface {
	GetID() string
	GetType() string
	GetVersion() int
	GetEvents() []Event
	ClearEvents()
	Apply(event interface{}) error
}

// NewAggregateBase creates a new aggregate base
func NewAggregateBase(aggregateType string, applier func(interface{}) error) *AggregateBase {
	return &AggregateBase{
		id:           uuid.New().String(),
		aggregateType: aggregateType,
		version:      0,
		events:       []Event{},
		applier:      applier,
	}
}

// GetID returns the aggregate ID
func (a *AggregateBase) GetID() string {
	return a.id
}

// SetID sets the aggregate ID
func (a *AggregateBase) SetID(id string) {
	a.id = id
}

// GetType returns the aggregate type
func (a *AggregateBase) GetType() string {
	return a.aggregateType
}

// GetVersion returns the aggregate version
func (a *AggregateBase) GetVersion() int {
	return a.version
}

// GetEvents returns the events
func (a *AggregateBase) GetEvents() []Event {
	return a.events
}

// ClearEvents clears the events
func (a *AggregateBase) ClearEvents() {
	a.events = []Event{}
}

// Apply applies an event to the aggregate
func (a *AggregateBase) Apply(event interface{}) error {
	if a.applier == nil {
		return fmt.Errorf("applier is not set")
	}

	// Apply the event to update the aggregate state
	if err := a.applier(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Create a new domain event
	domainEvent := Event{
		AggregateID:   a.id,
		AggregateType: a.aggregateType,
		Version:       a.version + 1,
		Timestamp:     time.Now(),
		Data:          event,
	}

	// Set the event type based on the event struct
	switch event.(type) {
	// Canister events
	case CanisterCreatedEvent:
		domainEvent.Type = CanisterCreated
	case CanisterUpdatedEvent:
		domainEvent.Type = CanisterUpdated
	case CanisterEntryEvent:
		domainEvent.Type = CanisterEntry
	case CanisterExitEvent:
		domainEvent.Type = CanisterExit
	case CanisterCheckEvent:
		domainEvent.Type = CanisterCheck
	case CanisterDamageEvent:
		domainEvent.Type = CanisterDamage
	case CanisterOrgCheckInEvent:
		domainEvent.Type = CanisterOrgCheckIn
	case CanisterOrgCheckOutEvent:
		domainEvent.Type = CanisterOrgCheckOut
	case CanisterRestoreDamageEvent:
		domainEvent.Type = CanisterRestoreDamage
	case CanisterRestoreTamperEvent:
		domainEvent.Type = CanisterRestoreTamper
	case CanisterRefillerEntryEvent:
		domainEvent.Type = CanisterRefillerEntry
	case CanisterRefillerExitEvent:
		domainEvent.Type = CanisterRefillerExit
	case CanisterRefillSessionEvent:
		domainEvent.Type = CanisterRefillSession
	
	// Delivery events
	case DeliveryNoteCreatedEvent:
		domainEvent.Type = DeliveryNoteCreated
	case DeliveryItemsAddedEvent:
		domainEvent.Type = DeliveryItemsAdded
	case DeliveryItemRemovedEvent:
		domainEvent.Type = DeliveryItemRemoved
	default:
		return fmt.Errorf("unknown event type: %T", event)
	}

	// Store the event
	a.events = append(a.events, domainEvent)
	
	// Increment version
	a.version++

	return nil
}