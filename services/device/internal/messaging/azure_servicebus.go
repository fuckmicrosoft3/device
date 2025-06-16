// internal/messaging/azure_servicebus.go
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"crypto/rand"
	"encoding/hex"
	
	"example.com/backstage/services/device/config"
	
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// ServiceBusClient is an interface for Azure Service Bus operations
type ServiceBusClient interface {
	SendMessage(ctx context.Context, body interface{}, sessionID string) error
	Close() error
}

// serviceBusClient implements the ServiceBusClient interface
type serviceBusClient struct {
	client     *azservicebus.Client
	sender     *azservicebus.Sender
	queueName  string
	clientType string
}

// NewServiceBusClient creates a new Azure Service Bus client
func NewServiceBusClient(cfg config.ServiceBusConfig, clientType string) (ServiceBusClient, error) {
	if cfg.ConnectionString == "" {
		return nil, fmt.Errorf("service bus connection string not provided")
	}
	
	// Create the Service Bus client
	client, err := azservicebus.NewClientFromConnectionString(cfg.ConnectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Service Bus client: %w", err)
	}
	
	// Create a sender for the queue
	sender, err := client.NewSender(cfg.QueueName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Service Bus sender: %w", err)
	}
	
	return &serviceBusClient{
		client:     client,
		sender:     sender,
		queueName:  cfg.QueueName,
		clientType: clientType,
	}, nil
}

// generateSessionID generates a random session ID if none is provided
func generateSessionID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// SendMessage sends a message to the Service Bus queue with retry logic
func (s *serviceBusClient) SendMessage(ctx context.Context, body interface{}, sessionID string) error {
	// Convert the body to JSON
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal message body: %w", err)
	}
	
	// Make sure we have a session ID
	if sessionID == "" {
		sessionID = generateSessionID()
	}
	
	// Create the message with a SessionId
	msg := &azservicebus.Message{
		Body: data,
		ApplicationProperties: map[string]interface{}{
			"source": s.clientType,
			"time":   time.Now().UTC().Format(time.RFC3339),
		},
		SessionID: &sessionID,
	}
	
	// Send the message with retries
	maxRetries := 3
	backoff := 500 * time.Millisecond
	
	for i := 0; i < maxRetries; i++ {
		err := s.sender.SendMessage(ctx, msg, nil)
		if err == nil {
			return nil
		}
		
		// If this is the last retry, return the error
		if i == maxRetries-1 {
			return fmt.Errorf("failed to send message after %d retries: %w", maxRetries, err)
		}
		
		// Wait with exponential backoff before retrying
		time.Sleep(backoff * time.Duration(1<<uint(i)))
	}
	
	return nil // This line should never be reached, but added for completeness
}

// Close closes the Service Bus client
func (s *serviceBusClient) Close() error {
	// Close the sender
	if s.sender != nil {
		if err := s.sender.Close(context.Background()); err != nil {
			return err
		}
	}
	
	// Close the client
	if s.client != nil {
		return s.client.Close(context.Background())
	}
	
	return nil
}
