// services/device/cmd/republish.go
package cmd

import (
	"context"
	"fmt"
	"time"

	"example.com/backstage/services/device/internal/core"
	"example.com/backstage/services/device/internal/infrastructure"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	republishDeviceUID   string
	republishStartTime   string
	republishEndTime     string
	republishAll         bool
	republishFailed      bool
	republishLimit       int
	republishDryRun      bool
	republishConcurrency int
)

var republishCmd = &cobra.Command{
	Use:   "republish",
	Short: "Republish telemetry messages from database to queues",
	Long: `Republish telemetry messages from the database to their configured queues.
This command is useful for recovering from queue outages or reprocessing messages.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRepublish()
	},
}

func init() {
	rootCmd.AddCommand(republishCmd)

	// Command flags
	republishCmd.Flags().StringVarP(&republishDeviceUID, "device", "d", "", "Device UID to republish messages for")
	republishCmd.Flags().StringVarP(&republishStartTime, "start", "s", "", "Start time (RFC3339 format, e.g., 2024-01-01T00:00:00Z)")
	republishCmd.Flags().StringVarP(&republishEndTime, "end", "e", "", "End time (RFC3339 format, e.g., 2024-01-02T00:00:00Z)")
	republishCmd.Flags().BoolVarP(&republishAll, "all", "a", false, "Republish all unprocessed messages")
	republishCmd.Flags().BoolVarP(&republishFailed, "failed", "f", false, "Republish only failed messages")
	republishCmd.Flags().IntVarP(&republishLimit, "limit", "l", 1000, "Maximum number of messages to process")
	republishCmd.Flags().BoolVar(&republishDryRun, "dry-run", false, "Show what would be republished without actually sending")
	republishCmd.Flags().IntVarP(&republishConcurrency, "concurrency", "c", 10, "Number of concurrent workers")
}

func runRepublish() error {
	logger.Info("Starting telemetry republish...")

	// Validate flags
	if !republishAll && !republishFailed && republishDeviceUID == "" && republishStartTime == "" {
		return fmt.Errorf("must specify at least one filter: --all, --failed, --device, or time range")
	}

	// Parse time range if provided
	var startTime, endTime *time.Time
	if republishStartTime != "" {
		t, err := time.Parse(time.RFC3339, republishStartTime)
		if err != nil {
			return fmt.Errorf("invalid start time format: %w", err)
		}
		startTime = &t
	}
	if republishEndTime != "" {
		t, err := time.Parse(time.RFC3339, republishEndTime)
		if err != nil {
			return fmt.Errorf("invalid end time format: %w", err)
		}
		endTime = &t
	}

	// Connect to infrastructure
	db, err := infrastructure.NewDatabase(cfg.Database)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	messaging, err := infrastructure.NewMessaging(cfg.ServiceBus)
	if err != nil {
		return fmt.Errorf("messaging connection failed: %w", err)
	}
	defer messaging.Close()

	// Create services
	dataStore := core.NewDataStore(db.DB)

	// Create republisher
	republisher := &TelemetryRepublisher{
		store:       dataStore,
		messaging:   messaging,
		logger:      logger,
		dryRun:      republishDryRun,
		concurrency: republishConcurrency,
	}

	// Build filter criteria
	criteria := RepublishCriteria{
		DeviceUID:   republishDeviceUID,
		StartTime:   startTime,
		EndTime:     endTime,
		AllMessages: republishAll,
		FailedOnly:  republishFailed,
		Limit:       republishLimit,
	}

	// Execute republish
	stats, err := republisher.Republish(context.Background(), criteria)
	if err != nil {
		return fmt.Errorf("republish failed: %w", err)
	}

	// Print results
	logger.WithFields(logrus.Fields{
		"total_processed": stats.TotalProcessed,
		"successful":      stats.Successful,
		"failed":          stats.Failed,
		"skipped":         stats.Skipped,
		"dry_run":         republishDryRun,
	}).Info("Republish completed")

	if stats.Failed > 0 {
		logger.Warnf("Failed to republish %d messages", stats.Failed)
	}

	return nil
}

// RepublishCriteria defines the filter criteria for republishing
type RepublishCriteria struct {
	DeviceUID   string
	StartTime   *time.Time
	EndTime     *time.Time
	AllMessages bool
	FailedOnly  bool
	Limit       int
}

// RepublishStats contains statistics about the republish operation
type RepublishStats struct {
	TotalProcessed int
	Successful     int
	Failed         int
	Skipped        int
}

// TelemetryRepublisher handles republishing telemetry messages
type TelemetryRepublisher struct {
	store       core.DataStore
	messaging   *infrastructure.Messaging
	logger      *logrus.Logger
	dryRun      bool
	concurrency int
}

// Republish processes messages based on criteria
func (r *TelemetryRepublisher) Republish(ctx context.Context, criteria RepublishCriteria) (*RepublishStats, error) {
	stats := &RepublishStats{}

	// Get messages based on criteria
	messages, err := r.getMessages(ctx, criteria)
	if err != nil {
		return stats, fmt.Errorf("failed to get messages: %w", err)
	}

	stats.TotalProcessed = len(messages)
	r.logger.Infof("Found %d messages to process", len(messages))

	if r.dryRun {
		r.logger.Info("DRY RUN: No messages will be sent")
		// Show sample of what would be processed
		for i, msg := range messages {
			if i >= 10 {
				r.logger.Infof("... and %d more messages", len(messages)-10)
				break
			}
			r.logger.WithFields(logrus.Fields{
				"message_id":     msg.MessageID,
				"device_id":      msg.DeviceID,
				"telemetry_type": msg.TelemetryType,
				"received_at":    msg.ReceivedAt,
			}).Info("Would republish message")
		}
		return stats, nil
	}

	// Process messages concurrently
	semaphore := make(chan struct{}, r.concurrency)
	results := make(chan bool, len(messages))

	for _, msg := range messages {
		semaphore <- struct{}{} // Acquire
		go func(telemetry *core.Telemetry) {
			defer func() { <-semaphore }() // Release
			success := r.processMessage(ctx, telemetry)
			results <- success
		}(msg)
	}

	// Wait for all goroutines to complete
	for i := 0; i < len(messages); i++ {
		if <-results {
			stats.Successful++
		} else {
			stats.Failed++
		}
	}

	return stats, nil
}

// getMessages retrieves messages based on criteria
func (r *TelemetryRepublisher) getMessages(ctx context.Context, criteria RepublishCriteria) ([]*core.Telemetry, error) {
	var messages []*core.Telemetry
	var err error

	if criteria.FailedOnly {
		// Get failed messages
		messages, err = r.store.GetFailedTelemetry(ctx, criteria.Limit)
	} else if criteria.AllMessages {
		// Get all unprocessed messages
		messages, err = r.store.GetUnprocessedTelemetry(ctx, criteria.Limit)
	} else if criteria.DeviceUID != "" && criteria.StartTime != nil && criteria.EndTime != nil {
		// Get messages for specific device and time range
		device, err := r.store.GetDeviceByUID(ctx, criteria.DeviceUID)
		if err != nil {
			return nil, fmt.Errorf("device not found: %w", err)
		}
		messages, err = r.store.GetTelemetryByTimeRange(ctx, device.ID, *criteria.StartTime, *criteria.EndTime)
		if err != nil {
			return nil, err
		}
		// Apply limit
		if len(messages) > criteria.Limit {
			messages = messages[:criteria.Limit]
		}
	} else if criteria.DeviceUID != "" {
		// Get messages for specific device
		device, err := r.store.GetDeviceByUID(ctx, criteria.DeviceUID)
		if err != nil {
			return nil, fmt.Errorf("device not found: %w", err)
		}
		messages, err = r.store.GetDeviceTelemetry(ctx, device.ID, criteria.Limit)
	} else {
		return nil, fmt.Errorf("invalid criteria combination")
	}

	if err != nil {
		return nil, err
	}

	return messages, nil
}

// processMessage processes a single telemetry message
func (r *TelemetryRepublisher) processMessage(ctx context.Context, telemetry *core.Telemetry) bool {
	// Get device to find organization
	device, err := r.store.GetDevice(ctx, telemetry.DeviceID)
	if err != nil {
		r.logger.WithError(err).WithField("device_id", telemetry.DeviceID).
			Error("Failed to get device")
		return false
	}

	// Get organization with queue mappings
	org, err := r.store.GetOrganization(ctx, device.OrganizationID)
	if err != nil {
		r.logger.WithError(err).WithField("org_id", device.OrganizationID).
			Error("Failed to get organization")
		return false
	}

	// Determine target queue
	queueName, err := org.QueueMappings.GetQueueForTelemetryType(telemetry.TelemetryType)
	if err != nil {
		r.logger.WithError(err).WithFields(logrus.Fields{
			"telemetry_type": telemetry.TelemetryType,
			"org_id":         device.OrganizationID,
			"message_id":     telemetry.MessageID,
		}).Error("No queue configured for telemetry type")

		// Update retry count
		r.store.UpdateTelemetryRetryCount(ctx, telemetry.MessageID, telemetry.RetryCount+1, err.Error())
		return false
	}

	// Publish to queue
	err = r.messaging.PublishToQueue(ctx, org.ConnectionString, queueName, telemetry)
	if err != nil {
		r.logger.WithError(err).WithFields(logrus.Fields{
			"queue":      queueName,
			"message_id": telemetry.MessageID,
		}).Error("Failed to publish message")

		// Update retry count
		r.store.UpdateTelemetryRetryCount(ctx, telemetry.MessageID, telemetry.RetryCount+1, err.Error())
		return false
	}

	// Mark as processed
	if err := r.store.MarkTelemetryProcessed(ctx, telemetry.MessageID); err != nil {
		r.logger.WithError(err).WithField("message_id", telemetry.MessageID).
			Warn("Failed to mark message as processed")
	}

	r.logger.WithFields(logrus.Fields{
		"message_id":     telemetry.MessageID,
		"queue":          queueName,
		"telemetry_type": telemetry.TelemetryType,
	}).Debug("Message republished successfully")

	return true
}
