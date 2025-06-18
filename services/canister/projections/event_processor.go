package projections

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"go.novek.io/canister/domain"
	"go.novek.io/canister/eventstore"
	"go.novek.io/canister/models"
)

// EventProcessor processes events from the database and projects them
type EventProcessor struct {
	db                 *gorm.DB
	canisterProjector  *CanisterProjector
	deliveryProjector  *DeliveryProjector
	eventStore         *eventstore.GormEventStore
	batchSize          int
	processingInterval time.Duration
	running            bool
	mutex              sync.Mutex
	stopChan           chan struct{}
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(
	db *gorm.DB,
	canisterProjector *CanisterProjector,
	deliveryProjector *DeliveryProjector,
	eventStore *eventstore.GormEventStore,
) *EventProcessor {
	return &EventProcessor{
		db:                 db,
		canisterProjector:  canisterProjector,
		deliveryProjector:  deliveryProjector,
		eventStore:         eventStore,
		batchSize:          100,
		processingInterval: 5 * time.Second,
		running:            false,
		stopChan:           make(chan struct{}),
	}
}

// Start starts the event processor
func (p *EventProcessor) Start() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		return
	}

	p.running = true
	go p.processEvents()
}

// Stop stops the event processor
func (p *EventProcessor) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.running {
		return
	}

	p.running = false
	p.stopChan <- struct{}{}
}

// processEvents processes events in a loop
func (p *EventProcessor) processEvents() {
	ticker := time.NewTicker(p.processingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := p.processBatch(); err != nil {
				log.Error().Err(err).Msg("Failed to process event batch")
			}
		case <-p.stopChan:
			return
		}
	}
}

// processBatch processes a batch of events
func (p *EventProcessor) processBatch() error {
	// Find unprocessed events
	var events []models.Event
	if err := p.db.Where("processed = ?", false).
		Order("timestamp ASC").
		Limit(p.batchSize).
		Find(&events).Error; err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	log.Info().Msgf("Processing %d events", len(events))

	// Process each event
	for _, event := range events {
		if err := p.processEvent(event); err != nil {
			log.Error().Err(err).Str("event_id", event.EventID).Msg("Failed to process event")
			// Update the event with the error
			errMsg := err.Error()
			p.db.Model(&event).Updates(map[string]interface{}{
				"error": &errMsg,
			})
			continue
		}

		// Mark the event as processed
		if err := p.db.Model(&event).Updates(map[string]interface{}{
			"processed": true,
			"error":     nil,
		}).Error; err != nil {
			log.Error().Err(err).Str("event_id", event.EventID).Msg("Failed to mark event as processed")
		}
	}

	return nil
}

// processEvent processes a single event
func (p *EventProcessor) processEvent(event models.Event) error {
	ctx := context.Background()

	// Convert to domain.Event
	domainEvent := domain.Event{
		ID:            event.EventID,
		AggregateID:   event.AggregateID,
		AggregateType: event.AggregateType,
		Type:          event.EventType,
		Version:       event.Version,
		Timestamp:     event.Timestamp,
		Data:          event.Data,
	}

	// Determine which projector to use based on the aggregate type
	switch event.AggregateType {
	case "canister":
		return p.canisterProjector.Project(ctx, domainEvent)
	case "delivery":
		return p.deliveryProjector.Project(ctx, domainEvent)
	default:
		log.Warn().Str("aggregate_type", event.AggregateType).Msg("Unknown aggregate type")
		return nil
	}
}
