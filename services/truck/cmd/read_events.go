package cmd

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go.novek.io/truck/config"
	"go.novek.io/truck/internal/db"
	"go.novek.io/truck/internal/messagebus"
	"go.novek.io/truck/internal/model"
	"go.novek.io/truck/internal/repository"
)

var (
	messageCount int
)

var readEventsCmd = &cobra.Command{
	Use:   "read_events",
	Short: "Read events from the message queue",
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			logrus.Fatalf("Failed to load configuration: %v", err)
		}

		// Connect to database
		dbConn, err := db.Connect(&cfg.Database)
		if err != nil {
			logrus.Fatalf("Failed to connect to database: %v", err)
		}

		// Initialize message bus
		messageBusClient, err := messagebus.NewClient(&cfg.MessageBus)
		if err != nil {
			logrus.Fatalf("Failed to initialize message bus: %v", err)
		}

		// Create repositories
		deviceRepo := repository.NewDeviceRepository(dbConn)
		operationRepo := repository.NewOperationRepository(dbConn)
		operationGroupRepo := repository.NewOperationGroupRepository(dbConn)

		// Create context
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Process each queue
		for _, queueName := range cfg.MessageBus.Queues {
			logrus.Infof("Reading from queue: %s", queueName)

			// Receive messages
			messages, err := messageBusClient.ReceiveMessages(ctx, queueName, messageCount)
			if err != nil {
				logrus.Errorf("Failed to read messages from queue %s: %v", queueName, err)
				continue
			}

			logrus.Infof("Received %d messages from queue %s", len(messages), queueName)

			// Process each message
			for _, message := range messages {
				if err := processMessage(ctx, message, deviceRepo, operationRepo, operationGroupRepo); err != nil {
					logrus.Errorf("Failed to process message: %v", err)
					if err := message.Reject(ctx); err != nil {
						logrus.Errorf("Failed to reject message: %v", err)
					}
				} else {
					if err := message.Complete(ctx); err != nil {
						logrus.Errorf("Failed to complete message: %v", err)
					}
				}
			}
		}

		// Close message bus
		if err := messageBusClient.Close(ctx); err != nil {
			logrus.Errorf("Failed to close message bus: %v", err)
		}
	},
}

func init() {
	readEventsCmd.Flags().IntVarP(&messageCount, "count", "c", 100, "Number of messages to read from each queue")
}

// processMessage processes a message from the queue
func processMessage(
	ctx context.Context,
	message messagebus.Message,
	deviceRepo repository.DeviceRepository,
	operationRepo repository.OperationRepository,
	operationGroupRepo repository.OperationGroupRepository,
) error {
	// Get message content
	content, err := message.GetMessage()
	if err != nil {
		return err
	}

	// Get truck UID
	truckUID, ok := content["truck_uid"]
	if !ok || truckUID == nil {
		return nil
	}

	// Find or create truck device
	truckDevice, err := deviceRepo.FindOrCreateTransportByMCU(ctx, truckUID.(string))
	if err != nil {
		return err
	}

	// Get operation group ID
	groupID, ok := content["uid"]
	if !ok || groupID == nil {
		return nil
	}

	// Create operation group
	group := &model.OperationGroup{
		Base: model.Base{
			UUID: groupID.(string),
		},
		TransportDeviceID: truckDevice.UUID,
		Type:              model.RefillOperationType,
		Status:            model.StatusFromString(content["status"].(string)),
	}

	// Parse created_at
	if createdAt, ok := content["created_at"].(string); ok {
		parsedTime, err := time.Parse("2006-01-02 15:04:05", createdAt)
		if err == nil {
			group.CreatedAt = parsedTime
		} else {
			group.CreatedAt = time.Now()
		}
	} else {
		group.CreatedAt = time.Now()
	}

	// Parse scheduled_at
	if scheduledAt, ok := content["scheduled_at"].(string); ok {
		parsedTime, err := time.Parse("2006-01-02 15:04:05", scheduledAt)
		if err == nil {
			group.ScheduledAt = &parsedTime
		}
	}

	// Save operation group
	group, err = operationGroupRepo.Create(ctx, group)
	if err != nil {
		return err
	}

	// Get operations
	operations, ok := content["operations"].([]interface{})
	if !ok {
		return nil
	}

	// Process operations
	for _, op := range operations {
		operationMap, ok := op.(map[string]interface{})
		if !ok {
			continue
		}

		// Get amount
		amount, ok := operationMap["amount"].(float64)
		if !ok {
			continue
		}

		// Get status
		status := model.StatusFromString(operationMap["status"].(string))

		// Get operation ID
		operationID, ok := operationMap["uid"]
		if !ok || operationID == nil {
			continue
		}

		// Get device ID
		deviceUID, ok := operationMap["device_uid"]
		if !ok || deviceUID == nil {
			continue
		}

		// Find or create device
		device, err := deviceRepo.FindOrCreateDeviceByMCU(ctx, deviceUID.(string))
		if err != nil {
			return err
		}

		// Create operation
		operation := &model.Operation{
			Base: model.Base{
				UUID:      operationID.(string),
				CreatedAt: group.CreatedAt,
			},
			DeviceID:           device.UUID,
			DeviceMCU:          device.MCU,
			TransportDeviceID:  truckDevice.UUID,
			TransportDeviceMCU: truckDevice.MCU,
			OperationGroupID:   group.UUID,
			Amount:             amount,
			Unit:               model.OperationUnitLitres,
			Type:               model.TypeFromString(operationMap["type"].(string)),
			Status:             status,
		}

		// Save operation
		_, err = operationRepo.Create(ctx, operation)
		if err != nil {
			return err
		}
	}

	// If all operations are complete, mark group as complete
	if group.Status == model.CompleteOperationStatus {
		return nil
	}

	// Check for active operations
	activeOps, err := operationRepo.FindAllActiveBy(ctx, model.Operation{
		OperationGroupID: group.UUID,
	})
	if err != nil {
		return err
	}

	// If no active operations, mark group as complete
	if len(activeOps) == 0 {
		group.Status = model.CompleteOperationStatus
		_, err = operationGroupRepo.Update(ctx, group)
		if err != nil {
			return err
		}
	}

	return nil
}
