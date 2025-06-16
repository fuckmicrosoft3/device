package eventstore

import (
	"context"
	"example.com/backstage/services/canister/domain"
)

// EventStore is the interface for event storage
type EventStore interface {
	// Save saves an aggregate's events to the store
	Save(ctx context.Context, aggregate domain.Aggregate) error
	
	// Load loads an aggregate from the store
	Load(ctx context.Context, aggregate domain.Aggregate) error
	
	// Exists checks if an aggregate exists
	Exists(ctx context.Context, aggregateID string) (bool, error)
	
	// GetEvents gets all events for an aggregate
	GetEvents(ctx context.Context, aggregateID string) ([]domain.Event, error)
	
	// GetUnprocessedEvents gets all unprocessed events
	GetUnprocessedEvents(ctx context.Context, limit int) ([]domain.Event, error)
	
	// MarkEventAsProcessed marks an event as processed
	MarkEventAsProcessed(ctx context.Context, eventID string) error
}