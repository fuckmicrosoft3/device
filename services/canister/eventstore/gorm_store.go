package eventstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"example.com/backstage/services/canister/domain"
	"example.com/backstage/services/canister/models"
)

// GormEventStore implements EventStore using GORM
type GormEventStore struct {
	db *gorm.DB
}

// NewGormEventStore creates a new GORM event store
func NewGormEventStore(db *gorm.DB) *GormEventStore {
	return &GormEventStore{db: db}
}

// Save saves an aggregate's events to the store
func (s *GormEventStore) Save(ctx context.Context, aggregate domain.Aggregate) error {
	// Get all uncommitted events
	events := aggregate.GetEvents()
	if len(events) == 0 {
		return nil
	}

	// Start a transaction
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, event := range events {
			// Marshal event data
			data, err := json.Marshal(event.Data)
			if err != nil {
				return fmt.Errorf("failed to marshal event data: %w", err)
			}

			// Create the event record
			dbEvent := models.Event{
				EventID:       uuid.New().String(),
				AggregateID:   event.AggregateID,
				AggregateType: event.AggregateType,
				EventType:     event.Type,
				Data:          data,
				Version:       event.Version,
				Timestamp:     event.Timestamp,
				Processed:     false,
			}

			// Save the event
			if err := tx.Create(&dbEvent).Error; err != nil {
				return fmt.Errorf("failed to save event: %w", err)
			}

			log.Info().
				Str("aggregateID", event.AggregateID).
				Str("eventType", event.Type).
				Int("version", event.Version).
				Msg("Event saved")
		}

		// Clear the events from the aggregate
		aggregate.ClearEvents()
		return nil
	})
}

// Load loads an aggregate from the store
func (s *GormEventStore) Load(ctx context.Context, aggregate domain.Aggregate) error {
	// Get the aggregate ID
	aggregateID := aggregate.GetID()
	if aggregateID == "" {
		return fmt.Errorf("aggregate ID is empty")
	}

	// Get all events for the aggregate
	var dbEvents []models.Event
	if err := s.db.WithContext(ctx).
		Where("aggregate_id = ?", aggregateID).
		Order("version ASC").
		Find(&dbEvents).Error; err != nil {
		return fmt.Errorf("failed to load events: %w", err)
	}

	// If there are no events, the aggregate doesn't exist
	if len(dbEvents) == 0 {
		return nil
	}

	// Process each event
	for _, dbEvent := range dbEvents {
		// Get the event data
		var eventData interface{}
		
		// Determine the event type and unmarshal accordingly
		switch dbEvent.EventType {
		// Canister events
		case domain.CanisterCreated:
			var data domain.CanisterCreatedEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.CanisterUpdated:
			var data domain.CanisterUpdatedEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.CanisterEntry:
			var data domain.CanisterEntryEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.CanisterExit:
			var data domain.CanisterExitEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.CanisterCheck:
			var data domain.CanisterCheckEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.CanisterDamage:
			var data domain.CanisterDamageEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.CanisterOrgCheckIn:
			var data domain.CanisterOrgCheckInEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.CanisterOrgCheckOut:
			var data domain.CanisterOrgCheckOutEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.CanisterRestoreDamage:
			var data domain.CanisterRestoreDamageEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.CanisterRestoreTamper:
			var data domain.CanisterRestoreTamperEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.CanisterRefillerEntry:
			var data domain.CanisterRefillerEntryEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.CanisterRefillerExit:
			var data domain.CanisterRefillerExitEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.CanisterRefillSession:
			var data domain.CanisterRefillSessionEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		// Delivery events
		case domain.DeliveryNoteCreated:
			var data domain.DeliveryNoteCreatedEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.DeliveryItemsAdded:
			var data domain.DeliveryItemsAddedEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		case domain.DeliveryItemRemoved:
			var data domain.DeliveryItemRemovedEvent
			if err := json.Unmarshal(dbEvent.Data, &data); err != nil {
				return fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			eventData = data
			
		default:
			return fmt.Errorf("unknown event type: %s", dbEvent.EventType)
		}

		// Apply the event to the aggregate
		if err := aggregate.Apply(eventData); err != nil {
			return fmt.Errorf("failed to apply event: %w", err)
		}
	}

	// Clear the events from the aggregate
	aggregate.ClearEvents()
	return nil
}

// Exists checks if an aggregate exists
func (s *GormEventStore) Exists(ctx context.Context, aggregateID string) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&models.Event{}).
		Where("aggregate_id = ?", aggregateID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if aggregate exists: %w", err)
	}

	return count > 0, nil
}

// GetEvents gets all events for an aggregate
func (s *GormEventStore) GetEvents(ctx context.Context, aggregateID string) ([]domain.Event, error) {
	var dbEvents []models.Event
	if err := s.db.WithContext(ctx).
		Where("aggregate_id = ?", aggregateID).
		Order("version ASC").
		Find(&dbEvents).Error; err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	// Convert to domain events
	events := make([]domain.Event, len(dbEvents))
	for i, dbEvent := range dbEvents {
		events[i] = domain.Event{
			ID:            dbEvent.EventID,
			AggregateID:   dbEvent.AggregateID,
			AggregateType: dbEvent.AggregateType,
			Type:          dbEvent.EventType,
			Version:       dbEvent.Version,
			Timestamp:     dbEvent.Timestamp,
		}
	}

	return events, nil
}

// GetUnprocessedEvents gets all unprocessed events
func (s *GormEventStore) GetUnprocessedEvents(ctx context.Context, limit int) ([]domain.Event, error) {
	var dbEvents []models.Event
	if err := s.db.WithContext(ctx).
		Where("processed = ?", false).
		Order("timestamp ASC").
		Limit(limit).
		Find(&dbEvents).Error; err != nil {
		return nil, fmt.Errorf("failed to get unprocessed events: %w", err)
	}

	// Convert to domain events
	events := make([]domain.Event, len(dbEvents))
	for i, dbEvent := range dbEvents {
		events[i] = domain.Event{
			ID:            dbEvent.EventID,
			AggregateID:   dbEvent.AggregateID,
			AggregateType: dbEvent.AggregateType,
			Type:          dbEvent.EventType,
			Version:       dbEvent.Version,
			Timestamp:     dbEvent.Timestamp,
		}
	}

	return events, nil
}

// MarkEventAsProcessed marks an event as processed
func (s *GormEventStore) MarkEventAsProcessed(ctx context.Context, eventID string) error {
	if err := s.db.WithContext(ctx).
		Model(&models.Event{}).
		Where("event_id = ?", eventID).
		Update("processed", true).
		Update("updated_at", time.Now()).
		Error; err != nil {
		return fmt.Errorf("failed to mark event as processed: %w", err)
	}

	return nil
}