package projections

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v7"
	"gorm.io/gorm"

	"go.novek.io/canister/config"
	"go.novek.io/canister/domain"
	"go.novek.io/canister/models"
)

// Constants for delivery indices
const (
	DeliveryNotesIndex = "delivery-notes"
	DeliveryItemsIndex = "delivery-items"
)

// DeliveryProjector handles projections for delivery events
type DeliveryProjector struct {
	db            *gorm.DB
	elasticClient *elasticsearch.Client
	cfg           config.Config
}

// NewDeliveryProjector creates a new delivery projector
func NewDeliveryProjector(db *gorm.DB, elasticClient *elasticsearch.Client, cfg config.Config) *DeliveryProjector {
	return &DeliveryProjector{
		db:            db,
		elasticClient: elasticClient,
		cfg:           cfg,
	}
}

// Project projects an event
func (p *DeliveryProjector) Project(ctx context.Context, event domain.Event) error {
	switch event.Type {
	case domain.DeliveryNoteCreated:
		return p.projectDeliveryNoteCreated(ctx, event)
	case domain.DeliveryItemsAdded:
		return p.projectDeliveryItemsAdded(ctx, event)
	case domain.DeliveryItemRemoved:
		return p.projectDeliveryItemRemoved(ctx, event)
	default:
		return nil
	}
}

// projectDeliveryNoteCreated handles the delivery note created event
func (p *DeliveryProjector) projectDeliveryNoteCreated(ctx context.Context, event domain.Event) error {
	// Parse event data
	var data domain.DeliveryNoteCreatedEvent
	if err := json.Unmarshal(event.Data.([]byte), &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	// Create delivery note in database
	deliveryNote := models.DeliveryNote{
		DeliveryNoteID: data.ID,
		AggregateID:    event.AggregateID,
		OrganizationID: data.OrganizationID,
		Status:         "Created",
		CreatedAt:      event.Timestamp,
		UpdatedAt:      event.Timestamp,
	}

	if err := p.db.WithContext(ctx).Create(&deliveryNote).Error; err != nil {
		return fmt.Errorf("failed to create delivery note in database: %w", err)
	}

	// Index in Elasticsearch
	deliveryNoteDoc, err := json.Marshal(deliveryNote)
	if err != nil {
		return fmt.Errorf("failed to marshal delivery note: %w", err)
	}

	// Index delivery note in Elasticsearch
	index := FormatIndex(DeliveryNotesIndex, p.cfg)
	res, err := p.elasticClient.Index(
		index,
		bytes.NewReader(deliveryNoteDoc),
		p.elasticClient.Index.WithDocumentID(data.ID),
		p.elasticClient.Index.WithRefresh("true"),
		p.elasticClient.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to index delivery note in Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to index delivery note in Elasticsearch: %s", res.String())
	}

	return nil
}

// projectDeliveryItemsAdded handles the delivery items added event
func (p *DeliveryProjector) projectDeliveryItemsAdded(ctx context.Context, event domain.Event) error {
	// Parse event data
	var data domain.DeliveryItemsAddedEvent
	if err := json.Unmarshal(event.Data.([]byte), &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	// Start a transaction
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range data.DeliveryItems {
			// Create delivery item in database
			deliveryItem := models.DeliveryItem{
				ItemID:         item.ID,
				DeliveryNoteID: item.DeliveryNoteID,
				CanisterID:     item.CanisterID,
				Delivered:      item.Delivered,
				CreatedAt:      event.Timestamp,
				UpdatedAt:      event.Timestamp,
			}

			if err := tx.Create(&deliveryItem).Error; err != nil {
				return fmt.Errorf("failed to create delivery item in database: %w", err)
			}

			// Index in Elasticsearch
			deliveryItemDoc, err := json.Marshal(deliveryItem)
			if err != nil {
				return fmt.Errorf("failed to marshal delivery item: %w", err)
			}

			// Index delivery item in Elasticsearch
			index := FormatIndex(DeliveryItemsIndex, p.cfg)
			res, err := p.elasticClient.Index(
				index,
				bytes.NewReader(deliveryItemDoc),
				p.elasticClient.Index.WithDocumentID(item.ID),
				p.elasticClient.Index.WithRefresh("true"),
				p.elasticClient.Index.WithContext(ctx),
			)
			if err != nil {
				return fmt.Errorf("failed to index delivery item in Elasticsearch: %w", err)
			}
			defer res.Body.Close()

			if res.IsError() {
				return fmt.Errorf("failed to index delivery item in Elasticsearch: %s", res.String())
			}
		}

		return nil
	})
}

// projectDeliveryItemRemoved handles the delivery item removed event
func (p *DeliveryProjector) projectDeliveryItemRemoved(ctx context.Context, event domain.Event) error {
	// Parse event data
	var data domain.DeliveryItemRemovedEvent
	if err := json.Unmarshal(event.Data.([]byte), &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	// Delete delivery item from database
	if err := p.db.WithContext(ctx).Delete(&models.DeliveryItem{}, "item_id = ?", data.ID).Error; err != nil {
		return fmt.Errorf("failed to delete delivery item from database: %w", err)
	}

	// Delete from Elasticsearch
	index := FormatIndex(DeliveryItemsIndex, p.cfg)
	res, err := p.elasticClient.Delete(
		index,
		data.ID,
		p.elasticClient.Delete.WithRefresh("true"),
		p.elasticClient.Delete.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to delete delivery item from Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != 404 { // Ignore 404 Not Found
		return fmt.Errorf("failed to delete delivery item from Elasticsearch: %s", res.String())
	}

	return nil
}
