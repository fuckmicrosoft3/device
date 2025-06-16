package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"example.com/backstage/services/sales/config"
	"example.com/backstage/services/sales/internal/models"
	"example.com/backstage/services/sales/internal/tracing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/google/uuid"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// AzureServiceBus provides Azure Service Bus integration
type AzureServiceBus struct {
	client *azservicebus.Client
	config config.AzureConfig
	tracer tracing.Tracer
}

// NewAzureServiceBus creates a new Azure Service Bus client
func NewAzureServiceBus(cfg config.AzureConfig, tracer tracing.Tracer) (*AzureServiceBus, error) {
	client, err := azservicebus.NewClientFromConnectionString(cfg.QueueConnStr, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Azure Service Bus client")
	}

	return &AzureServiceBus{
		client: client,
		config: cfg,
		tracer: tracer,
	}, nil
}

// MainMessage struct that holds the entire service bus message
type MainMessage struct {
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

// MessageProcessor defines a function type for processing messages
type MessageProcessor func(ctx context.Context, message *azservicebus.ReceivedMessage, txn *newrelic.Transaction) error

// ProcessMessages starts listening for messages and processing them
func (a *AzureServiceBus) ProcessMessages(ctx context.Context, processor MessageProcessor) error {
	receiverOptions := &azservicebus.ReceiverOptions{
		ReceiveMode: azservicebus.ReceiveModePeekLock,
	}

	receiver, err := a.client.NewReceiverForQueue(a.config.QueueName, receiverOptions)
	if err != nil {
		return errors.Wrap(err, "failed to create receiver for queue")
	}
	defer func() {
		if err := receiver.Close(context.Background()); err != nil {
			log.Error().Err(err).Msg("error closing receiver")
		}
	}()

	log.Info().Msgf("Listening for messages on queue: %s", a.config.QueueName)

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Shutting down Azure Service Bus receiver")
			return nil

		default:
			messages, err := receiver.ReceiveMessages(ctx, 100, nil)
			if err != nil {
				log.Error().Err(err).Msg("error receiving messages")
				// Sleep briefly to prevent tight loop in case of persistent errors
				time.Sleep(time.Second)
				continue
			}

			log.Info().Msgf("Received %d messages", len(messages))

			for _, message := range messages {
				// Start a transaction for this message
				txn := a.tracer.StartTransaction(fmt.Sprintf("process-message-%s", message.MessageID))
				
				// Process the message
				err := processor(ctx, message, txn)
				if err != nil {
					log.Error().Err(err).Str("messageID", message.MessageID).Msg("error processing message")
					a.tracer.RecordError(txn, err)
					
					// Abandon the message so it can be retried
					if abandonErr := receiver.AbandonMessage(context.Background(), message, nil); abandonErr != nil {
						log.Error().Err(abandonErr).Str("messageID", message.MessageID).Msg("error abandoning message")
					}
				} else {
					// Complete the message
					if completeErr := receiver.CompleteMessage(context.Background(), message, nil); completeErr != nil {
						log.Error().Err(completeErr).Str("messageID", message.MessageID).Msg("error completing message")
					}
				}
				
				// End the transaction
				a.tracer.EndTransaction(txn)
			}

			// If no messages, wait a moment before polling again
			if len(messages) == 0 {
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
}

// ExtractDispenseDetails extracts dispense details from a message
func ExtractDispenseDetails(message *azservicebus.ReceivedMessage) (*models.SalePayload, error) {
	var mainMessage MainMessage
	if err := json.Unmarshal(message.Body, &mainMessage); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal main message")
	}

	var payload models.SalePayload
	if err := json.Unmarshal([]byte(mainMessage.Payload), &payload); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal payload")
	}

	// Set device from MCU if not present in payload
	if payload.Device == "" && mainMessage.MCU != "" {
		payload.Device = mainMessage.MCU
	}

	// Set event type if not present in payload
	if payload.EventType == "" && mainMessage.EventType != "" {
		payload.EventType = mainMessage.EventType
	}

	return &payload, nil
}

// Close closes the Azure Service Bus client
func (a *AzureServiceBus) Close() error {
	return nil // No explicit close method in the Azure SDK client
}

// GetIdempotencyKey generates an idempotency key
func GetIdempotencyKey() uuid.UUID {
	return uuid.New()
}