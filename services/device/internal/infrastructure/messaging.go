// services/device/internal/infrastructure/messaging.go
package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"example.com/backstage/services/device/config"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/sirupsen/logrus"
)

type Messaging struct {
	client        *azservicebus.Client
	sender        *azservicebus.Sender
	maxRetries    int
	retryDelay    time.Duration
	deadLetterWAL *WAL
	logger        *logrus.Logger
}

func NewMessaging(cfg config.ServiceBusConfig) (*Messaging, error) {
	if cfg.ConnectionString == "" {
		return nil, fmt.Errorf("service bus connection string is required")
	}

	client, err := azservicebus.NewClientFromConnectionString(cfg.ConnectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create service bus client: %w", err)
	}

	sender, err := client.NewSender(cfg.QueueName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sender: %w", err)
	}

	// Initialize dead letter WAL for failed messages
	deadLetterWAL, err := NewWAL("/data/messaging-dead-letter")
	if err != nil {
		// Log but don't fail - messaging can work without WAL
		logrus.WithError(err).Warn("Failed to initialize dead letter WAL")
	}

	return &Messaging{
		client:        client,
		sender:        sender,
		maxRetries:    cfg.MaxRetries,
		retryDelay:    cfg.RetryDelay,
		deadLetterWAL: deadLetterWAL,
		logger:        logrus.New(),
	}, nil
}

// Publish sends a message without retry
func (m *Messaging) Publish(ctx context.Context, topic string, message interface{}) error {
	return m.publishMessage(ctx, topic, message)
}

// PublishWithRetry sends a message with retry logic
func (m *Messaging) PublishWithRetry(ctx context.Context, topic string, message interface{}, maxRetries int) error {
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
				"topic":   topic,
			}).Warn("Retrying message publish")

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := m.publishMessage(ctx, topic, message)
		if err == nil {
			if attempt > 0 {
				m.logger.WithFields(logrus.Fields{
					"attempt": attempt,
					"topic":   topic,
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
			"topic":     topic,
			"message":   message,
			"error":     lastErr.Error(),
			"timestamp": time.Now(),
			"retries":   maxRetries,
		}

		if err := m.deadLetterWAL.Write(deadLetterEntry); err != nil {
			m.logger.WithError(err).Error("Failed to write to dead letter WAL")
		} else {
			m.logger.WithFields(logrus.Fields{
				"topic": topic,
				"error": lastErr,
			}).Error("Message sent to dead letter queue after max retries")
		}
	}

	return fmt.Errorf("failed to publish message after %d retries: %w", maxRetries, lastErr)
}

// publishMessage handles the actual message publishing
func (m *Messaging) publishMessage(ctx context.Context, topic string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	msg := &azservicebus.Message{
		Body: data,
		ApplicationProperties: map[string]interface{}{
			"topic":     topic,
			"timestamp": time.Now().Unix(),
		},
		ContentType: strPtr("application/json"),
		Subject:     &topic,
	}

	// Add message ID for deduplication
	if msgWithID, ok := message.(interface{ GetMessageID() string }); ok {
		messageID := msgWithID.GetMessageID()
		msg.MessageID = &messageID
	}

	return m.sender.SendMessage(ctx, msg, nil)
}

// PublishBatch sends multiple messages efficiently
func (m *Messaging) PublishBatch(ctx context.Context, topic string, messages []interface{}) error {
	if len(messages) == 0 {
		return nil
	}

	batch, err := m.sender.NewMessageBatch(ctx, nil)
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
				"topic":     topic,
				"timestamp": time.Now().Unix(),
			},
		}

		err = batch.AddMessage(msg, nil)
		if err != nil {
			// Batch is full, send it
			if err := m.sender.SendMessageBatch(ctx, batch, nil); err != nil {
				return fmt.Errorf("failed to send message batch: %w", err)
			}

			// Create new batch
			batch, err = m.sender.NewMessageBatch(ctx, nil)
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
		if err := m.sender.SendMessageBatch(ctx, batch, nil); err != nil {
			return fmt.Errorf("failed to send final message batch: %w", err)
		}
	}

	// Retry failed messages individually
	for _, msg := range failedMessages {
		if err := m.PublishWithRetry(ctx, topic, msg, 3); err != nil {
			m.logger.WithError(err).Error("Failed to publish message from batch")
		}
	}

	return nil
}

// isRetryableError determines if an error should trigger a retry
func (m *Messaging) isRetryableError(err error) bool {
	// Add specific error checks for Azure Service Bus
	// For now, we'll retry most errors except context cancellation
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	return true
}

// RecoverDeadLetters attempts to reprocess messages from dead letter queue
func (m *Messaging) RecoverDeadLetters(ctx context.Context) error {
	if m.deadLetterWAL == nil {
		return fmt.Errorf("dead letter WAL not initialized")
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

		topic, _ := deadLetter["topic"].(string)
		message := deadLetter["message"]

		if err := m.Publish(ctx, topic, message); err != nil {
			failed++
			m.logger.WithError(err).WithField("topic", topic).
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

// Stats returns messaging statistics
func (m *Messaging) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"connected": m.sender != nil,
	}

	if m.deadLetterWAL != nil {
		stats["dead_letter"] = m.deadLetterWAL.Stats()
	}

	return stats
}

// Close gracefully shuts down the messaging client
func (m *Messaging) Close() error {
	var errors []error

	if m.sender != nil {
		if err := m.sender.Close(context.Background()); err != nil {
			errors = append(errors, fmt.Errorf("failed to close sender: %w", err))
		}
	}

	if m.client != nil {
		if err := m.client.Close(context.Background()); err != nil {
			errors = append(errors, fmt.Errorf("failed to close client: %w", err))
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

// Helper function to get string pointer
func strPtr(s string) *string {
	return &s
}
