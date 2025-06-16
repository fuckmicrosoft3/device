package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"example.com/backstage/services/device/config"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

type Messaging struct {
	client *azservicebus.Client
	sender *azservicebus.Sender
}

func NewMessaging(cfg config.ServiceBusConfig) (*Messaging, error) {
	client, err := azservicebus.NewClientFromConnectionString(cfg.ConnectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create service bus client: %w", err)
	}

	sender, err := client.NewSender(cfg.QueueName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sender: %w", err)
	}

	return &Messaging{
		client: client,
		sender: sender,
	}, nil
}

func (m *Messaging) Publish(ctx context.Context, topic string, message interface{}) error {
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
	}

	return m.sender.SendMessage(ctx, msg, nil)
}

func (m *Messaging) Close() error {
	if m.sender != nil {
		if err := m.sender.Close(context.Background()); err != nil {
			return err
		}
	}

	if m.client != nil {
		return m.client.Close(context.Background())
	}

	return nil
}
