// services/device/internal/infrastructure/messaging.go
package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/sirupsen/logrus"
	"go.novek.io/device/config"
)

type Messaging struct {
	clients       map[string]*azservicebus.Client // connectionString -> client
	senders       map[string]*azservicebus.Sender // queueName -> sender
	mu            sync.RWMutex
	maxRetries    int
	retryDelay    time.Duration
	deadLetterWAL *WAL
	logger        *logrus.Logger
}

func NewMessaging(cfg config.ServiceBusConfig) (*Messaging, error) {
	// Initialize dead letter WAL for failed messages
	deadLetterWAL, err := NewWAL("/data/messaging-dead-letter")
	if err != nil {
		// Log but don't fail - messaging can work without WAL
		logrus.WithError(err).Warn("Failed to initialize dead letter WAL")
	}

	return &Messaging{
		clients:       make(map[string]*azservicebus.Client),
		senders:       make(map[string]*azservicebus.Sender),
		maxRetries:    cfg.MaxRetries,
		retryDelay:    cfg.RetryDelay,
		deadLetterWAL: deadLetterWAL,
		logger:        logrus.New(),
	}, nil
}

// InitializeQueues pre-initializes connections for configured queues.
func (m *Messaging) InitializeQueues(queueConfigs []config.QueueConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, qc := range queueConfigs {
		// Get or create client for this connection string
		client, exists := m.clients[qc.ConnectionString]
		if !exists {
			newClient, err := azservicebus.NewClientFromConnectionString(qc.ConnectionString, nil)
			if err != nil {
				m.logger.WithError(err).WithField("organization", qc.OrganizationName).
					Error("Failed to create service bus client")
				continue
			}
			m.clients[qc.ConnectionString] = newClient
			client = newClient
		}

		// Create senders for each queue
		for _, route := range qc.QueueRoutes {
			if _, exists := m.senders[route.QueueName]; !exists {
				sender, err := client.NewSender(route.QueueName, nil)
				if err != nil {
					m.logger.WithError(err).WithField("queue", route.QueueName).
						Error("Failed to create sender")
					continue
				}
				m.senders[route.QueueName] = sender
				m.logger.WithField("queue", route.QueueName).Info("Initialized queue sender")
			}
		}
	}

	return nil
}

// PublishToQueue sends a message to a specific queue.
func (m *Messaging) PublishToQueue(ctx context.Context, queueName string, message interface{}, maxRetries int) error {
	m.mu.RLock()
	sender, exists := m.senders[queueName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no sender configured for queue: %s", queueName)
	}

	var lastErr error

	// Use configured max retries if not specified
	if maxRetries <= 0 {
		maxRetries = m.maxRetries
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := time.Duration(attempt) * m.retryDelay
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}

			m.logger.WithFields(logrus.Fields{
				"attempt": attempt,
				"delay":   delay,
				"queue":   queueName,
			}).Warn("Retrying message publish")

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := m.publishMessageToQueue(ctx, sender, queueName, message)
		if err == nil {
			if attempt > 0 {
				m.logger.WithFields(logrus.Fields{
					"attempt": attempt,
					"queue":   queueName,
				}).Info("Message published successfully after retry")
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !m.isRetryableError(err) {
			break
		}
	}

	// All retries failed, send to dead letter
	if m.deadLetterWAL != nil {
		deadLetterEntry := map[string]interface{}{
			"queue":     queueName,
			"message":   message,
			"error":     lastErr.Error(),
			"timestamp": time.Now(),
			"retries":   maxRetries,
		}

		if err := m.deadLetterWAL.Write(deadLetterEntry); err != nil {
			m.logger.WithError(err).Error("Failed to write to dead letter WAL")
		} else {
			m.logger.WithFields(logrus.Fields{
				"queue": queueName,
				"error": lastErr,
			}).Error("Message sent to dead letter queue after max retries")
		}
	}

	return fmt.Errorf("failed to publish message to queue %s after %d retries: %w", queueName, maxRetries, lastErr)
}

// publishMessageToQueue handles the actual message publishing.
func (m *Messaging) publishMessageToQueue(ctx context.Context, sender *azservicebus.Sender, queueName string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Extract telemetry type if available for message properties
	var telemetryType string
	if telemetry, ok := message.(interface{ GetTelemetryType() string }); ok {
		telemetryType = telemetry.GetTelemetryType()
	}

	msg := &azservicebus.Message{
		Body: data,
		ApplicationProperties: map[string]interface{}{
			"queue":          queueName,
			"timestamp":      time.Now().Unix(),
			"telemetry_type": telemetryType,
		},
		ContentType: strPtr("application/json"),
		Subject:     &queueName,
	}

	// Add message ID for deduplication
	if msgWithID, ok := message.(interface{ GetID() string }); ok {
		messageID := msgWithID.GetID()
		msg.MessageID = &messageID
	}

	return sender.SendMessage(ctx, msg, nil)
}

// Publish sends a message without retry (legacy support).
func (m *Messaging) Publish(ctx context.Context, topic string, message interface{}) error {
	// For backward compatibility - route to a default queue if needed
	return errors.New("legacy Publish method not supported - use PublishToQueue")
}

// PublishWithRetry sends a message with retry logic (legacy support).
func (m *Messaging) PublishWithRetry(ctx context.Context, topic string, message interface{}, maxRetries int) error {
	// For backward compatibility - route to a default queue if needed
	return errors.New("legacy PublishWithRetry method not supported - use PublishToQueue")
}

// PublishBatchToQueue sends multiple messages efficiently to a specific queue.
func (m *Messaging) PublishBatchToQueue(ctx context.Context, queueName string, messages []interface{}) error {
	if len(messages) == 0 {
		return nil
	}

	m.mu.RLock()
	sender, exists := m.senders[queueName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no sender configured for queue: %s", queueName)
	}

	batch, err := sender.NewMessageBatch(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create message batch: %w", err)
	}

	failedMessages := []interface{}{}

	for _, message := range messages {
		data, err := json.Marshal(message)
		if err != nil {
			m.logger.WithError(err).Warn("Failed to marshal message in batch")
			failedMessages = append(failedMessages, message)
			continue
		}

		msg := &azservicebus.Message{
			Body: data,
			ApplicationProperties: map[string]interface{}{
				"queue":     queueName,
				"timestamp": time.Now().Unix(),
			},
		}

		err = batch.AddMessage(msg, nil)
		if err != nil {
			// Batch is full, send it
			if err := sender.SendMessageBatch(ctx, batch, nil); err != nil {
				return fmt.Errorf("failed to send message batch: %w", err)
			}

			// Create new batch
			batch, err = sender.NewMessageBatch(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to create new message batch: %w", err)
			}

			// Try adding message to new batch
			if err := batch.AddMessage(msg, nil); err != nil {
				m.logger.WithError(err).Warn("Failed to add message to new batch")
				failedMessages = append(failedMessages, message)
			}
		}
	}

	// Send remaining messages
	if batch.NumMessages() > 0 {
		if err := sender.SendMessageBatch(ctx, batch, nil); err != nil {
			return fmt.Errorf("failed to send final message batch: %w", err)
		}
	}

	// Retry failed messages individually
	for _, msg := range failedMessages {
		if err := m.PublishToQueue(ctx, queueName, msg, 3); err != nil {
			m.logger.WithError(err).Error("Failed to publish message from batch")
		}
	}

	return nil
}

// isRetryableError determines if an error should trigger a retry.
func (m *Messaging) isRetryableError(err error) bool {
	// Add specific error checks for Azure Service Bus
	// For now, we'll retry most errors except context cancellation
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	return true
}

// RecoverDeadLetters attempts to reprocess messages from dead letter queue.
func (m *Messaging) RecoverDeadLetters(ctx context.Context) error {
	if m.deadLetterWAL == nil {
		return errors.New("dead letter WAL not initialized")
	}

	entries, err := m.deadLetterWAL.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read dead letter entries: %w", err)
	}

	recovered := 0
	failed := 0

	for _, entry := range entries {
		deadLetter, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}

		queue, _ := deadLetter["queue"].(string)
		message := deadLetter["message"]

		if err := m.PublishToQueue(ctx, queue, message, 1); err != nil {
			failed++
			m.logger.WithError(err).WithField("queue", queue).
				Warn("Failed to recover dead letter message")
		} else {
			recovered++
		}
	}

	m.logger.WithFields(logrus.Fields{
		"recovered": recovered,
		"failed":    failed,
		"total":     len(entries),
	}).Info("Dead letter recovery completed")

	return nil
}

// Stats returns messaging statistics.
func (m *Messaging) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"clients_count": len(m.clients),
		"senders_count": len(m.senders),
	}

	if m.deadLetterWAL != nil {
		stats["dead_letter"] = m.deadLetterWAL.Stats()
	}

	// List configured queues
	queues := make([]string, 0, len(m.senders))
	for queueName := range m.senders {
		queues = append(queues, queueName)
	}
	stats["configured_queues"] = queues

	return stats
}

// Close gracefully shuts down the messaging client.
func (m *Messaging) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	// Close all senders
	for queueName, sender := range m.senders {
		if err := sender.Close(context.Background()); err != nil {
			errors = append(errors, fmt.Errorf("failed to close sender for queue %s: %w", queueName, err))
		}
	}

	// Close all clients
	for _, client := range m.clients {
		if err := client.Close(context.Background()); err != nil {
			errors = append(errors, fmt.Errorf("failed to close service bus client: %w", err))
		}
	}

	if m.deadLetterWAL != nil {
		if err := m.deadLetterWAL.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close dead letter WAL: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during close: %v", errors)
	}

	return nil
}

// Helper function to get string pointer.
func strPtr(s string) *string {
	return &s
}
