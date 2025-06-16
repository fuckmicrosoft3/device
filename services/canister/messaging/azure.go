package messaging

import (
	"context"
	"errors"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/rs/zerolog/log"

	"example.com/backstage/services/canister/config"
)

type AzureClient struct {
	client *azservicebus.Client
}

func NewAzureClient(cfg config.Config) (*AzureClient, error) {
	client, err := azservicebus.NewClientFromConnectionString(cfg.AzureQueueConnStr, nil)
	if err != nil {
		return nil, err
	}

	return &AzureClient{client: client}, nil
}

func (a *AzureClient) StartConsumers(queueName string, processor MessageProcessor) error {
	log.Info().Msgf("Starting consumers for queue %s", queueName)

	// Loop continuously to handle reconnections
	for {
		sessionReceiver, err := a.client.AcceptNextSessionForQueue(context.TODO(), queueName, nil)
		if err != nil {
			var sbErr *azservicebus.Error
			if errors.As(err, &sbErr) && sbErr.Code == azservicebus.CodeTimeout {
				log.Info().Msg("No session available, waiting...")
				time.Sleep(2 * time.Second)
				continue
			}
			return err
		}

		log.Info().Msgf("Session '%s' received", sessionReceiver.SessionID())

		go a.handleSession(sessionReceiver, processor)
	}
}

func (a *AzureClient) handleSession(receiver *azservicebus.SessionReceiver, processor MessageProcessor) {
	defer func() {
		log.Info().Msgf("Closing session '%s'", receiver.SessionID())
		err := receiver.Close(context.TODO())
		if err != nil {
			log.Error().Err(err).Msgf("Error closing session '%s'", receiver.SessionID())
		}
	}()

	// Process messages in batches
	for {
		messages, err := receiver.ReceiveMessages(context.TODO(), 10, nil)

		if err != nil {
			log.Error().Err(err).Msgf("Error receiving messages from session '%s'", receiver.SessionID())
			return
		}

		if len(messages) == 0 {
			// No more messages in this session
			return
		}

		log.Info().Msgf("Received %d messages from session '%s'", len(messages), receiver.SessionID())

		for _, message := range messages {
			err := processor.ProcessMessage(context.Background(), message)
			if err != nil {
				log.Error().Err(err).Msgf("Error processing message '%s'", message.MessageID)
				// Return the message to the queue
				err = receiver.AbandonMessage(context.Background(), message, nil)
				if err != nil {
					log.Error().Err(err).Msgf("(AbandonMessage) err: %v", err)
				}
				continue
			}

			err = receiver.CompleteMessage(context.Background(), message, nil)
			if err != nil {
				log.Error().Err(err).Msgf("(CompleteMessage) err: %v", err)
			}
		}
	}
}