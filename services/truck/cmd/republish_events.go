package cmd

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go.novek.io/truck/config"
	"go.novek.io/truck/internal/cache"
	"go.novek.io/truck/internal/db"
	"go.novek.io/truck/internal/messagebus"
	"go.novek.io/truck/internal/model"
	"go.novek.io/truck/internal/repository"
	"go.novek.io/truck/internal/service"
)

var (
	startTime      string
	endTime        string
	operationGroup string
)

var republishEventsCmd = &cobra.Command{
	Use:   "republish_events",
	Short: "Republish events to the message queue",
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			logrus.Fatalf("Failed to load configuration: %v", err)
		}

		// Parse time range
		start, err := time.Parse(time.DateTime, startTime)
		if err != nil {
			logrus.Fatalf("Failed to parse start time: %v", err)
		}

		var end time.Time
		if endTime == "" {
			end = time.Now()
		} else {
			end, err = time.Parse(time.DateTime, endTime)
			if err != nil {
				logrus.Fatalf("Failed to parse end time: %v", err)
			}
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

		// Initialize cache
		cacheClient, err := cache.NewRedisClient(&cfg.Redis)
		if err != nil {
			logrus.Fatalf("Failed to connect to Redis: %v", err)
		}

		// Create repositories
		deviceRepo := repository.NewDeviceRepository(dbConn)
		operationRepo := repository.NewOperationRepository(dbConn)

		// Create service
		operationService := service.NewOperationService(
			operationRepo,
			deviceRepo,
			messageBusClient,
			cacheClient,
			cfg.MessageBus.ERPQueue,
		)

		// Create filter
		filter := model.OperationSession{}
		if operationGroup != "" {
			filter.OperationGroupID = operationGroup
		}

		// Create context
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		// Republish events
		count, err := operationService.RepublishEvents(ctx, start, end, filter)
		if err != nil {
			logrus.Fatalf("Failed to republish events: %v", err)
		}

		logrus.Infof("Republished %d events", count)

		// Close message bus
		if err := messageBusClient.Close(ctx); err != nil {
			logrus.Errorf("Failed to close message bus: %v", err)
		}
	},
}

func init() {
	// Default to 24 hours ago
	defaultStart := time.Now().Add(-24 * time.Hour).Format(time.DateTime)

	republishEventsCmd.Flags().StringVarP(&startTime, "start", "s", defaultStart, "Start time for republishing events (format: 2006-01-02T15:04:05)")
	republishEventsCmd.Flags().StringVarP(&endTime, "end", "e", "", "End time for republishing events (format: 2006-01-02T15:04:05)")
	republishEventsCmd.Flags().StringVarP(&operationGroup, "group", "g", "", "Operation group ID")
}
