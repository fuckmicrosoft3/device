package messagebus

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	"go.novek.io/truck/config"
	"go.novek.io/truck/internal/metrics"
)

// Client defines the interface for message bus operations
type Client interface {
	PublishMessage(ctx context.Context, message interface{}, queueName string) error
	ReceiveMessages(ctx context.Context, queueName string, count int) ([]Message, error)
	Close(ctx context.Context) error
}

// Message represents a message from the message bus
type Message interface {
	GetID() (string, error)
	GetMessage() (map[string]interface{}, error)
	Complete(ctx context.Context) error
	Reject(ctx context.Context) error
}

// AzureServiceBusClient implements Client using Azure Service Bus
type AzureServiceBusClient struct {
	client           *azservicebus.Client
	connectionString string
	prefix           string
}

// serviceBusMessage implements Message
type serviceBusMessage struct {
	message  *azservicebus.ReceivedMessage
	receiver *azservicebus.Receiver
	content  map[string]interface{}
}

// NewClient creates a new message bus client
func NewClient(cfg *config.MessageBusConfig) (Client, error) {
	// Create the client
	client, err := azservicebus.NewClientFromConnectionString(cfg.ConnectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create service bus client: %w", err)
	}

	return &AzureServiceBusClient{
		client:           client,
		connectionString: cfg.ConnectionString,
		prefix:           cfg.Prefix,
	}, nil
}

// getQueueName returns the full queue name with prefix
func (c *AzureServiceBusClient) getQueueName(queueName string) string {
	if c.prefix == "" {
		return queueName
	}
	return fmt.Sprintf("%s-%s", c.prefix, queueName)
}

// PublishMessage publishes a message to a queue
func (c *AzureServiceBusClient) PublishMessage(ctx context.Context, message interface{}, queueName string) error {
	// Record metrics
	startTime := time.Now()
	collector := metrics.GetMetricsCollector()

	// Get the sender for the queue
	sender, err := c.client.NewSender(c.getQueueName(queueName), nil)
	if err != nil {
		collector.RecordMessageBusOperation(metrics.MessageBusOperationSend, false, time.Since(startTime))
		return fmt.Errorf("failed to create sender for queue %s: %w", queueName, err)
	}
	defer sender.Close(ctx)

	// Marshal the message to JSON
	messageBytes, err := json.Marshal(message)
	if err != nil {
		collector.RecordMessageBusOperation(metrics.MessageBusOperationSend, false, time.Since(startTime))
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create a new message
	sbMessage := &azservicebus.Message{
		Body: messageBytes,
	}

	// Send the message
	if err := sender.SendMessage(ctx, sbMessage, nil); err != nil {
		collector.RecordMessageBusOperation(metrics.MessageBusOperationSend, false, time.Since(startTime))
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Record successful operation metrics
	collector.RecordMessageBusOperation(metrics.MessageBusOperationSend, true, time.Since(startTime))
	return nil
}

// ReceiveMessages receives messages from a queue
func (c *AzureServiceBusClient) ReceiveMessages(ctx context.Context, queueName string, count int) ([]Message, error) {
	// Record metrics
	startTime := time.Now()
	collector := metrics.GetMetricsCollector()

	// Get the receiver for the queue
	receiver, err := c.client.NewReceiverForQueue(
		c.getQueueName(queueName),
		&azservicebus.ReceiverOptions{
			ReceiveMode: azservicebus.ReceiveModePeekLock,
		},
	)
	if err != nil {
		collector.RecordMessageBusOperation(metrics.MessageBusOperationReceive, false, time.Since(startTime))
		return nil, fmt.Errorf("failed to create receiver for queue %s: %w", queueName, err)
	}

	// Receive messages with a timeout
	receiveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	sbMessages, err := receiver.ReceiveMessages(receiveCtx, count, nil)
	if err != nil {
		_ = receiver.Close(ctx)
		collector.RecordMessageBusOperation(metrics.MessageBusOperationReceive, false, time.Since(startTime))
		return nil, fmt.Errorf("failed to receive messages: %w", err)
	}

	// Wrap the messages
	messages := make([]Message, len(sbMessages))
	for i, sbMessage := range sbMessages {
		messages[i] = &serviceBusMessage{
			message:  sbMessage,
			receiver: receiver,
		}
	}

	// If no messages were received, close the receiver
	if len(messages) == 0 {
		_ = receiver.Close(ctx)
	}

	// Record number of messages received
	collector.RecordMessageBusOperation(metrics.MessageBusOperationReceive, true, time.Since(startTime))
	collector.SetPendingMessages(len(messages))

	return messages, nil
}

// Close closes the client
func (c *AzureServiceBusClient) Close(ctx context.Context) error {
	return nil // Client doesn't need explicit closing
}

// GetID gets the ID of the message
func (m *serviceBusMessage) GetID() (string, error) {
	content, err := m.GetMessage()
	if err != nil {
		return "", err
	}

	// Try different ID fields
	for _, field := range []string{"id", "exchanger_uuid", "uid"} {
		if id, ok := content[field].(string); ok {
			return id, nil
		}
	}

	return "", fmt.Errorf("message does not have an ID field")
}

// GetMessage gets the content of the message
func (m *serviceBusMessage) GetMessage() (map[string]interface{}, error) {
	if m.content != nil {
		return m.content, nil
	}

	var content map[string]interface{}
	if err := json.Unmarshal(m.message.Body, &content); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	m.content = content
	return content, nil
}

// Complete marks the message as complete
func (m *serviceBusMessage) Complete(ctx context.Context) error {
	// Record metrics
	startTime := time.Now()
	collector := metrics.GetMetricsCollector()

	if err := m.receiver.CompleteMessage(ctx, m.message, nil); err != nil {
		collector.RecordMessageBusOperation(metrics.MessageBusOperationComplete, false, time.Since(startTime))
		return fmt.Errorf("failed to complete message: %w", err)
	}

	collector.RecordMessageBusOperation(metrics.MessageBusOperationComplete, true, time.Since(startTime))
	return nil
}

// Reject rejects the message
func (m *serviceBusMessage) Reject(ctx context.Context) error {
	// Record metrics
	startTime := time.Now()
	collector := metrics.GetMetricsCollector()

	if err := m.receiver.AbandonMessage(ctx, m.message, nil); err != nil {
		collector.RecordMessageBusOperation(metrics.MessageBusOperationReject, false, time.Since(startTime))
		return fmt.Errorf("failed to abandon message: %w", err)
	}

	collector.RecordMessageBusOperation(metrics.MessageBusOperationReject, true, time.Since(startTime))
	return nil
}

// IsDisconnectionError checks if an error is a disconnection error
func IsDisconnectionError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	return strings.Contains(errMsg, "amqp: link detached") ||
		strings.Contains(errMsg, "awaiting send: context deadline exceeded")
}

// RetryWithBackoff retries an operation with exponential backoff
func RetryWithBackoff(ctx context.Context, fn func() error, maxRetries int) error {
	var err error

	for retry := 0; retry < maxRetries; retry++ {
		err = fn()
		if err == nil {
			return nil
		}

		if !IsDisconnectionError(err) {
			return err
		}

		// Calculate backoff duration
		backoff := time.Duration(1<<uint(retry)) * time.Second
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}

		// Wait for backoff duration or context cancellation
		select {
		case <-time.After(backoff):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return err
}
