// internal/service/message_processor.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"example.com/backstage/services/device/internal/cache"
	"example.com/backstage/services/device/internal/messaging"
	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/repository"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// MessageProcessor handles asynchronous message processing
type MessageProcessor struct {
	repo                    repository.Repository
	cache                   cache.RedisClient
	messagingClient         messaging.ServiceBusClient
	log                     *logrus.Logger
	workers                 int
	queue                   chan *models.DeviceMessage
	wg                      sync.WaitGroup
	ctx                     context.Context
	cancel                  context.CancelFunc
	queueCapacityAlertThreshold float64
}

// NewMessageProcessor creates a new message processor with worker pool
func NewMessageProcessor(
	repo repository.Repository,
	cache cache.RedisClient,
	messagingClient messaging.ServiceBusClient,
	log *logrus.Logger,
	workers int,
) *MessageProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	mp := &MessageProcessor{
		repo:                    repo,
		cache:                   cache,
		messagingClient:         messagingClient,
		log:                     log,
		workers:                 workers,
		queue:                   make(chan *models.DeviceMessage, 10000), // Buffer size
		ctx:                     ctx,
		cancel:                  cancel,
		queueCapacityAlertThreshold: 0.8, // 80% by default
	}
	
	// Start worker pool
	mp.startWorkers()
	
	// Start queue monitoring
	go mp.monitorQueueCapacity()
	
	mp.log.Infof("Started message processor with %d workers", workers)
	
	return mp
}

// startWorkers launches the worker goroutines
func (mp *MessageProcessor) startWorkers() {
	for i := 0; i < mp.workers; i++ {
		mp.wg.Add(1)
		go mp.worker(i)
	}
}

// worker processes messages from the queue
func (mp *MessageProcessor) worker(id int) {
	defer mp.wg.Done()
	
	for {
		select {
		case <-mp.ctx.Done():
			mp.log.Debugf("Worker %d shutting down", id)
			return
		case msg := <-mp.queue:
			start := time.Now()
			mp.processMessage(msg)
			mp.log.Debugf("Worker %d processed message in %v", id, time.Since(start))
		}
	}
}

// monitorQueueCapacity monitors the queue capacity and logs warnings when threshold is exceeded
func (mp *MessageProcessor) monitorQueueCapacity() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-mp.ctx.Done():
			return
		case <-ticker.C:
			queueLength := len(mp.queue)
			queueCapacity := cap(mp.queue)
			usage := float64(queueLength) / float64(queueCapacity)
			
			if usage >= mp.queueCapacityAlertThreshold {
				mp.log.Warnf("Message queue at %d%% capacity (%d/%d)!", int(usage*100), queueLength, queueCapacity)
				// Could trigger external alerting here
			}
		}
	}
}

// updateDeviceCache updates the device cache with retry logic
func (mp *MessageProcessor) updateDeviceCache(ctx context.Context, device *models.Device) {
	if device == nil {
		return
	}
	
	cacheKey := fmt.Sprintf("device:%s", device.UID)
	deviceJSON, err := json.Marshal(device)
	if err != nil {
		mp.log.WithError(err).Warnf("Failed to marshal device for cache: %s", device.UID)
		return
	}
	
	maxRetries := 3
	backoff := 100 * time.Millisecond
	
	for i := 0; i < maxRetries; i++ {
		err := mp.cache.Set(ctx, cacheKey, string(deviceJSON), 24*time.Hour)
		if err == nil {
			mp.log.Debugf("Updated device cache for: %s", device.UID)
			return
		}
		
		mp.log.WithError(err).Warnf("Failed to update device cache (attempt %d/%d): %s", i+1, maxRetries, device.UID)
		
		// If not the last retry, try again with backoff
		if i < maxRetries-1 {
			time.Sleep(backoff * time.Duration(1<<uint(i))) // Exponential backoff
			continue
		}
		
		mp.log.WithError(err).Errorf("Failed to update device cache after all retries: %s", device.UID)
	}
}

// processMessage handles the actual message processing with improved transaction support
func (mp *MessageProcessor) processMessage(message *models.DeviceMessage) {
	ctx := context.Background()
	
	// Try to get device from cache
	cacheKey := fmt.Sprintf("device:%s", message.DeviceMCU)
	var device *models.Device
	var err error
	
	// Try to get from cache first
	cachedData, err := mp.cache.Get(ctx, cacheKey)
	if err == nil {
		var cachedDevice models.Device
		if err := json.Unmarshal([]byte(cachedData), &cachedDevice); err == nil {
			device = &cachedDevice
			mp.log.Debugf("Device found in cache: %s", message.DeviceMCU)
		}
	}
	
	// If not in cache, get from database
	if device == nil {
		device, err = mp.repo.FindDeviceByUID(ctx, message.DeviceMCU)
		if err != nil {
			message.Error = true
			message.ErrorMessage = fmt.Sprintf("Device not found: %s", message.DeviceMCU)
			mp.log.Warnf("Device not found: %s", message.DeviceMCU)
		} else {
			message.DeviceID = device.ID
			message.Device = device
			
			// Update cache with new improved method
			go mp.updateDeviceCache(context.Background(), device)
		}
	} else {
		message.DeviceID = device.ID
		message.Device = device
	}
	
	// Use transaction for database operations
	err = mp.repo.WithTransaction(ctx, func(txCtx context.Context, txRepo repository.Repository) error {
		// Save message to database
		if err := txRepo.SaveDeviceMessage(txCtx, message); err != nil {
			return fmt.Errorf("failed to save message: %w", err)
		}
		
		// If device was found, publish the message
		if !message.Error {
			// Use device UID as the session ID
			sessionID := message.DeviceMCU
			
			// Publish to Service Bus
			if err := mp.messagingClient.SendMessage(txCtx, message, sessionID); err != nil {
				return fmt.Errorf("failed to publish message: %w", err)
			}
			
			// Mark as published
			now := time.Now()
			message.Published = true
			message.PublishedAt = &now
			
			if err := txRepo.MarkMessageAsPublished(txCtx, message.UUID); err != nil {
				return fmt.Errorf("failed to mark message as published: %w", err)
			}
		}
		
		return nil
	})
	
	if err != nil {
		mp.log.WithError(err).Error("Failed to process message")
	}
}

// EnqueueMessage adds a message to the queue for processing
func (mp *MessageProcessor) EnqueueMessage(message *models.DeviceMessage) error {
	// Ensure message has UUID
	if message.UUID == "" {
		message.UUID = uuid.New().String()
	}
	
	select {
	case mp.queue <- message:
		return nil
	default:
		// Queue is full, return error
		return errors.New("message queue is full")
	}
}

// Stop gracefully shuts down the processor
func (mp *MessageProcessor) Stop() {
	mp.log.Info("Stopping message processor...")
	mp.cancel()
	mp.wg.Wait()
	mp.log.Info("Message processor stopped")
}

// QueueStats returns current queue statistics
func (mp *MessageProcessor) QueueStats() map[string]interface{} {
	return map[string]interface{}{
		"queue_length":   len(mp.queue),
		"queue_capacity": cap(mp.queue),
		"worker_count":   mp.workers,
	}
}