package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"example.com/backstage/services/sales/internal/models"
	"example.com/backstage/services/sales/internal/store"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// MessageProcessor handles Service Bus message processing
type MessageProcessor struct {
	server  *Server
	message *azservicebus.ReceivedMessage
}

// ExternalMessage represents the outer message structure
type ExternalMessage struct {
	ID            int     `json:"ID"`
	CreatedAt     string  `json:"CreatedAt"`
	UpdatedAt     string  `json:"UpdatedAt"`
	DeletedAt     *string `json:"DeletedAt"`
	ExchangerUUID string  `json:"exchanger_uuid"`
	EventType     string  `json:"ev"`
	Serial        string  `json:"serial"`
	MCU           string  `json:"mcu"`
	DeviceID      string  `json:"device_id"`
	Source        string  `json:"source"`
	SourceID      string  `json:"source_id"`
	SourceTopic   string  `json:"source_topic"`
	Payload       string  `json:"payload"`
	Duplicate     bool    `json:"duplicate"`
	Time          string  `json:"time"`
}

// DispensePayload represents the nested dispense payload
type DispensePayload struct {
	Amount         int32     `json:"a"`
	AVol           int       `json:"a_vol"`
	DVol           float64   `json:"d_vol"`
	Device         string    `json:"device"`
	Dt             int       `json:"dt"`
	EVol           float64   `json:"e_vol"`
	EventType      string    `json:"ev"`
	ID             int       `json:"id"`
	Ms             int       `json:"ms"`
	P              int       `json:"p"`
	RVol           float64   `json:"r_vol"`
	S              int       `json:"s"`
	T              int32     `json:"t"`
	Tag            string    `json:"tag"`
	Timestamp      time.Time `json:"timestamp"`
	Topic          string    `json:"topic"`
	IdempotencyKey uuid.UUID `json:"u"`
}

// Process processes a Service Bus message
func (p *MessageProcessor) Process(ctx context.Context) error {
	messageID := ""
	if p.message.MessageID != nil {
		messageID = *p.message.MessageID
	}

	log.Debug().
		Str("message_id", messageID).
		Msg("Processing Service Bus message")

	// Use message deduplicator
	return p.server.messageDedup.ProcessOnce(messageID, func() error {
		// Parse external message
		var externalMsg ExternalMessage
		if err := json.Unmarshal(p.message.Body, &externalMsg); err != nil {
			return fmt.Errorf("failed to unmarshal external message: %w", err)
		}

		// Check event type - we only process "dispense" for now
		if externalMsg.EventType != "dispense" {
			log.Debug().
				Str("event_type", externalMsg.EventType).
				Msg("Skipping non-dispense event")
			return nil
		}

		// Parse inner payload
		var payload DispensePayload
		if err := json.Unmarshal([]byte(externalMsg.Payload), &payload); err != nil {
			return fmt.Errorf("failed to unmarshal dispense payload: %w", err)
		}

		// Process the dispense payload
		return p.processDispensePayload(ctx, &payload)
	})
}

// processDispensePayload processes a dispense payload
func (p *MessageProcessor) processDispensePayload(ctx context.Context, payload *DispensePayload) error {
	// Validate payload
	if err := p.validateDispensePayload(payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	// Check idempotency cache
	if result, found := p.server.idempotencyCache.Get(payload.IdempotencyKey); found {
		log.Debug().
			Str("idempotency_key", payload.IdempotencyKey.String()).
			Msg("Payload already processed (cache hit)")
		return nil
	}

	// Create dispense session
	session := &models.DispenseSession{
		EventType:        payload.EventType,
		ExpectedDispense: payload.EVol,
		RemainingVolume:  payload.RVol,
		AmountKSH:        payload.Amount,
		IdempotencyKey:   payload.IdempotencyKey,
		Time:             &payload.T,
		DeviceMCU:        &payload.Device,
		TotalPumpRuntime: int64(payload.Ms),
		EnrichmentStatus: models.EnrichmentStatusPending,
	}

	// Save to database
	err := p.server.store.CreateDispenseSession(ctx, session)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateKey) {
			log.Debug().
				Str("idempotency_key", payload.IdempotencyKey.String()).
				Msg("Payload already processed (database duplicate)")
			return nil
		}
		return fmt.Errorf("failed to create dispense session: %w", err)
	}

	// Cache the result
	p.server.idempotencyCache.Set(payload.IdempotencyKey, session.ID, 24*time.Hour)

	log.Info().
		Str("session_id", session.ID.String()).
		Str("idempotency_key", payload.IdempotencyKey.String()).
		Int32("amount", payload.Amount).
		Msg("Dispense session created from Service Bus message")

	// Queue for enrichment asynchronously
	go func() {
		enrichCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := p.server.enricher.EnrichDispenseSession(enrichCtx, session); err != nil {
			log.Error().
				Err(err).
				Str("session_id", session.ID.String()).
				Msg("Failed to enrich dispense session from Service Bus")
		} else {
			// Index the sale if enrichment succeeded
			sale, err := p.server.store.GetSaleByDispenseSessionID(enrichCtx, session.ID)
			if err == nil {
				p.server.indexer.IndexSale(sale.ID)
			}
		}
	}()

	return nil
}

// validateDispensePayload validates a dispense payload
func (p *MessageProcessor) validateDispensePayload(payload *DispensePayload) error {
	if payload.IdempotencyKey == uuid.Nil {
		return errors.New("idempotency key is required")
	}

	if payload.Device == "" {
		return errors.New("device MCU is required")
	}

	if payload.T == 0 {
		return errors.New("sale time is required")
	}

	if payload.Amount < 0 {
		return errors.New("amount cannot be negative")
	}

	return nil
}
