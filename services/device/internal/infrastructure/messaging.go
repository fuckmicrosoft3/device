// services/device/internal/infrastructure/messaging.go
package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"example.com/backstage/services/device/config"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/sirupsen/logrus"
)

// Messaging handles Azure Service Bus connections
type Messaging struct {
	defaultClient *azservicebus.Client
	defaultSender *azservicebus.Sender
	orgClients    map[string]*azservicebus.Client            // Connection string -> Client
	orgSenders    map[string]map[string]*azservicebus.Sender // Connection string -> Queue -> Sender
	mu            sync.RWMutex
	logger        *logrus.Logger
}

// NewMessaging creates a new messaging service
func NewMessaging(cfg config.ServiceBusConfig) (*Messaging, error) {
	m := &Messaging{
		orgClients: make(map[string]*azservicebus.Client),
		orgSenders: make(map[string]map[string]*azservicebus.Sender),
		logger:     logrus.New(),
	}

	// Initialize default client if configured
	if cfg.ConnectionString != "" {
		client, err := azservicebus.NewClientFromConnectionString(cfg.ConnectionString, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create default service bus client: %w", err)
		}
		m.defaultClient = client

		if cfg.QueueName != "" {
			sender, err := client.NewSender(cfg.QueueName, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create default sender: %w", err)
			}
			m.defaultSender = sender
		}
	}

	return m, nil
}

// getOrCreateClient gets or creates a client for a connection string
func (m *Messaging) getOrCreateClient(connectionString string) (*azservicebus.Client, error) {
	m.mu.RLock()
	client, exists := m.orgClients[connectionString]
	m.mu.RUnlock()

	if exists {
		return client, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	client, exists = m.orgClients[connectionString]
	if exists {
		return client, nil
	}

	// Create new client
	client, err := azservicebus.NewClientFromConnectionString(connectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create service bus client: %w", err)
	}

	m.orgClients[connectionString] = client
	return client, nil
}

// getOrCreateSender gets or creates a sender for a specific queue
func (m *Messaging) getOrCreateSender(connectionString, queueName string) (*azservicebus.Sender, error) {
	client, err := m.getOrCreateClient(connectionString)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	senders, exists := m.orgSenders[connectionString]
	if exists {
		sender, senderExists := senders[queueName]
		if senderExists {
			m.mu.RUnlock()
			return sender, nil
		}
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize map if needed
	if m.orgSenders[connectionString] == nil {
		m.orgSenders[connectionString] = make(map[string]*azservicebus.Sender)
	}

	// Double-check after acquiring write lock
	sender, exists := m.orgSenders[connectionString][queueName]
	if exists {
		return sender, nil
	}

	// Create new sender
	sender, err = client.NewSender(queueName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sender for queue %s: %w", queueName, err)
	}

	m.orgSenders[connectionString][queueName] = sender
	return sender, nil
}

// Publish sends a message to the default queue
func (m *Messaging) Publish(ctx context.Context, topic string, message interface{}) error {
	if m.defaultSender == nil {
		return fmt.Errorf("default sender not configured")
	}

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

	return m.defaultSender.SendMessage(ctx, msg, nil)
}

// PublishToQueue sends a message to a specific queue using organization's connection string
func (m *Messaging) PublishToQueue(ctx context.Context, connectionString, queueName string, message interface{}) error {
	if connectionString == "" {
		return fmt.Errorf("connection string is required")
	}
	if queueName == "" {
		return fmt.Errorf("queue name is required")
	}

	sender, err := m.getOrCreateSender(connectionString, queueName)
	if err != nil {
		return err
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Extract telemetry type if available
	telemetryType := ""
	if msg, ok := message.(interface{ GetTelemetryType() string }); ok {
		telemetryType = msg.GetTelemetryType()
	}

	sbMsg := &azservicebus.Message{
		Body: data,
		ApplicationProperties: map[string]interface{}{
			"telemetry_type": telemetryType,
			"timestamp":      time.Now().Unix(),
			"queue_name":     queueName,
		},
		ContentType: strPtr("application/json"),
	}

	// Add message ID for deduplication
	if msgWithID, ok := message.(interface{ GetMessageID() string }); ok {
		messageID := msgWithID.GetMessageID()
		sbMsg.MessageID = &messageID
	}

	return sender.SendMessage(ctx, sbMsg, nil)
}

// PublishBatchToQueue sends multiple messages to a specific queue
func (m *Messaging) PublishBatchToQueue(ctx context.Context, connectionString, queueName string, messages []interface{}) error {
	if len(messages) == 0 {
		return nil
	}

	sender, err := m.getOrCreateSender(connectionString, queueName)
	if err != nil {
		return err
	}

	batch, err := sender.NewMessageBatch(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create message batch: %w", err)
	}

	successCount := 0
	for _, message := range messages {
		data, err := json.Marshal(message)
		if err != nil {
			m.logger.WithError(err).Warn("Failed to marshal message in batch")
			continue
		}

		msg := &azservicebus.Message{
			Body: data,
			ApplicationProperties: map[string]interface{}{
				"timestamp":  time.Now().Unix(),
				"queue_name": queueName,
			},
		}

		err = batch.AddMessage(msg, nil)
		if err != nil {
			// Batch is full, send it
			if err := sender.SendMessageBatch(ctx, batch, nil); err != nil {
				return fmt.Errorf("failed to send message batch: %w", err)
			}
			m.logger.WithField("count", successCount).Debug("Sent partial batch")

			// Create new batch
			batch, err = sender.NewMessageBatch(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to create new message batch: %w", err)
			}

			// Try adding message to new batch
			if err := batch.AddMessage(msg, nil); err != nil {
				m.logger.WithError(err).Warn("Failed to add message to new batch")
				continue
			}
		}
		successCount++
	}

	// Send remaining messages
	if batch.NumMessages() > 0 {
		if err := sender.SendMessageBatch(ctx, batch, nil); err != nil {
			return fmt.Errorf("failed to send final message batch: %w", err)
		}
		m.logger.WithField("count", batch.NumMessages()).Debug("Sent final batch")
	}

	m.logger.WithFields(logrus.Fields{
		"total":   len(messages),
		"success": successCount,
		"queue":   queueName,
	}).Info("Batch published to queue")

	return nil
}

// Stats returns messaging statistics
func (m *Messaging) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"default_connected":    m.defaultClient != nil,
		"organization_clients": len(m.orgClients),
		"total_senders":        0,
	}

	// Count total senders
	for _, senders := range m.orgSenders {
		stats["total_senders"] = stats["total_senders"].(int) + len(senders)
	}

	return stats
}

// Close gracefully shuts down all messaging clients
func (m *Messaging) Close() error {
	var errors []error

	// Close default sender and client
	if m.defaultSender != nil {
		if err := m.defaultSender.Close(context.Background()); err != nil {
			errors = append(errors, fmt.Errorf("failed to close default sender: %w", err))
		}
	}

	if m.defaultClient != nil {
		if err := m.defaultClient.Close(context.Background()); err != nil {
			errors = append(errors, fmt.Errorf("failed to close default client: %w", err))
		}
	}

	// Close all organization senders
	m.mu.Lock()
	defer m.mu.Unlock()

	for connStr, senders := range m.orgSenders {
		for queueName, sender := range senders {
			if err := sender.Close(context.Background()); err != nil {
				errors = append(errors, fmt.Errorf("failed to close sender for queue %s: %w", queueName, err))
			}
		}
		delete(m.orgSenders, connStr)
	}

	// Close all organization clients
	for connStr, client := range m.orgClients {
		if err := client.Close(context.Background()); err != nil {
			errors = append(errors, fmt.Errorf("failed to close client: %w", err))
		}
		delete(m.orgClients, connStr)
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during close: %v", errors)
	}

	return nil
}

// GetMessageID implementation for Telemetry
func (t *Telemetry) GetMessageID() string {
	return t.MessageID
}

// GetTelemetryType implementation for Telemetry
func (t *Telemetry) GetTelemetryType() string {
	return t.TelemetryType
}

// Import Telemetry type to avoid circular dependency
type Telemetry interface {
	GetMessageID() string
	GetTelemetryType() string
}

// Helper function to get string pointer
func strPtr(s string) *string {
	return &s
}
