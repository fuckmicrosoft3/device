package handlers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"go.novek.io/canister/domain"
	"go.novek.io/canister/eventstore"
)

// Command structs
type CreateDeliveryNoteCommand struct {
	AggregateID    string `json:"aggregate_id"`
	OrganizationID string `json:"organization_id"`
}

type AddDeliveryItemsCommand struct {
	AggregateID   string                `json:"aggregate_id"`
	DeliveryItems []domain.DeliveryItem `json:"delivery_items"`
}

type RemoveDeliveryItemCommand struct {
	AggregateID string `json:"aggregate_id"`
	ItemID      string `json:"item_id"`
}

// DeliveryHandler handles all delivery-related commands
type DeliveryHandler struct {
	store eventstore.EventStore
}

// NewDeliveryHandler creates a new delivery handler
func NewDeliveryHandler(store eventstore.EventStore) *DeliveryHandler {
	return &DeliveryHandler{store: store}
}

// HandleCreateDeliveryNote creates a new delivery note
func (h *DeliveryHandler) HandleCreateDeliveryNote(ctx context.Context, cmd CreateDeliveryNoteCommand) error {

	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CreateDeliveryNote command")

	// Check if delivery note already exists
	exists, err := h.store.Exists(ctx, cmd.AggregateID)
	if err != nil {
		return fmt.Errorf("failed to check if delivery note exists: %w", err)
	}

	if exists {
		return fmt.Errorf("delivery note already exists with ID %s", cmd.AggregateID)
	}

	// Create a new aggregate
	aggregate := domain.NewDeliveryNoteAggregate(cmd.AggregateID)

	// Create the event
	event := domain.DeliveryNoteCreatedEvent{
		ID:             cmd.AggregateID,
		OrganizationID: cmd.OrganizationID,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleAddDeliveryItems adds items to a delivery note
func (h *DeliveryHandler) HandleAddDeliveryItems(ctx context.Context, cmd AddDeliveryItemsCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling AddDeliveryItems command")

	// Load the aggregate
	aggregate := domain.NewDeliveryNoteAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.DeliveryItemsAddedEvent{
		DeliveryItems: cmd.DeliveryItems,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleRemoveDeliveryItem removes an item from a delivery note
func (h *DeliveryHandler) HandleRemoveDeliveryItem(ctx context.Context, cmd RemoveDeliveryItemCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling RemoveDeliveryItem command")

	// Load the aggregate
	aggregate := domain.NewDeliveryNoteAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.DeliveryItemRemovedEvent{
		ID: cmd.ItemID,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}
