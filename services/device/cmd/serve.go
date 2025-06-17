// services/device/cmd/serve.go
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/backstage/services/device/config"
	"example.com/backstage/services/device/internal/api"
	"example.com/backstage/services/device/internal/core"
	"example.com/backstage/services/device/internal/infrastructure"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts the Device Management API server",
	Long:  `Launches the HTTP server and MQTT client to handle device registration, telemetry ingestion, and OTA updates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServer()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServer() error {
	logger.Info("Initializing Device Management Service...")

	// --- Infrastructure Setup ---
	logger.Info("Connecting to database...")
	db, err := infrastructure.NewDatabase(cfg.Database)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	logger.Info("Connecting to cache...")
	cache, err := infrastructure.NewCache(cfg.Redis)
	if err != nil {
		return fmt.Errorf("cache connection failed: %w", err)
	}
	defer cache.Close()

	logger.Info("Connecting to messaging service...")
	messaging, err := infrastructure.NewMessaging(cfg.ServiceBus)
	if err != nil {
		logger.Warn("Messaging service unavailable, continuing without it")
		messaging = nil
	} else {
		defer messaging.Close()
	}

	// --- Data Store Setup ---
	dataStore := core.NewDataStore(db.DB)

	// --- Individual Service Initialization ---

	// Device Management Service
	deviceManagement := core.NewDeviceManagementService(dataStore, cache, logger)

	// Telemetry Service
	telemetry := core.NewTelemetryService(dataStore, messaging, logger)
	defer telemetry.Stop() // Ensure proper shutdown

	// Firmware Management Service
	firmwareManagement, err := core.NewFirmwareManagementService(dataStore, logger, cfg.Firmware)
	if err != nil {
		return fmt.Errorf("failed to initialize firmware service: %w", err)
	}

	// Update Management Service
	updateManagement := core.NewUpdateManagementService(dataStore, firmwareManagement, logger, cfg.OTA)

	// Wire the circular dependency
	firmwareManagement.SetUpdateManagementService(updateManagement)

	// Organization Service
	organization := core.NewOrganizationService(dataStore, logger)

	// Authentication Service
	authentication := core.NewAuthenticationService(dataStore, logger)

	// Create service registry for API handlers
	services := &core.ServiceRegistry{
		DeviceManagement:   deviceManagement,
		Telemetry:          telemetry,
		FirmwareManagement: firmwareManagement,
		UpdateManagement:   updateManagement,
		Organization:       organization,
		Authentication:     authentication,
	}

	// Initialize pre-configured organizations
	if err := initializeOrganizations(ctx, organization, cfg.Organizations); err != nil {
		logger.WithError(err).Warn("Failed to initialize some organizations")
	}

	// --- MQTT Setup for Telemetry Ingestion ---
	mqttSubscriber, err := setupMQTTSubscriber(telemetry, logger)
	if err != nil {
		logger.WithError(err).Error("Failed to setup MQTT subscriber")
		// Continue without MQTT if it fails
		mqttSubscriber = nil
	} else {
		defer mqttSubscriber.Stop()
	}

	// --- API Layer Setup ---
	if gin.Mode() == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	handlers := api.NewAPIHandlers(services)
	api.SetupRoutes(router, handlers, services, logger)

	// --- HTTP Server ---
	serverAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// --- Graceful Shutdown ---
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Infof("Device Management API listening on %s", serverAddr)
		logger.Info("Service started successfully")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-shutdownChan

	logger.Warn("Shutdown signal received, initiating graceful shutdown...")

	// Create shutdown context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown order is important

	// 1. Stop accepting new MQTT messages
	if mqttSubscriber != nil {
		logger.Info("Stopping MQTT subscriber...")
		mqttSubscriber.Stop()
	}

	// 2. Stop telemetry processor
	logger.Info("Stopping telemetry processor...")
	telemetry.Stop()

	// 3. Shutdown HTTP server
	logger.Info("Shutting down HTTP server...")
	if err := server.Shutdown(ctx); err != nil {
		logger.Errorf("Server shutdown failed: %v", err)
	} else {
		logger.Info("Server stopped gracefully")
	}

	// 4. Close infrastructure connections (deferred above)

	logger.Info("Device Management Service shutdown complete")
	return nil
}

func initializeOrganizations(ctx context.Context, orgService *core.OrganizationService, configs []config.OrganizationConfig) error {
	for _, orgConfig := range configs {
		logger.WithField("organization", orgConfig.Name).Info("Initializing organization")

		// Check if organization already exists
		orgs, err := orgService.ListOrganizations(ctx)
		if err != nil {
			logger.WithError(err).Error("Failed to list organizations")
			continue
		}

		var existingOrg *core.Organization
		for _, org := range orgs {
			if org.Name == orgConfig.Name {
				existingOrg = org
				break
			}
		}

		// Build queue mappings
		queueMappings := core.QueueMappings{
			Queues: make([]core.QueueMapping, 0, len(orgConfig.Queues)),
		}
		for _, queueConfig := range orgConfig.Queues {
			queueMappings.Queues = append(queueMappings.Queues, core.QueueMapping{
				QueueName:      queueConfig.Name,
				TelemetryTypes: queueConfig.Types,
			})
		}

		if existingOrg != nil {
			// Update existing organization
			existingOrg.ConnectionString = orgConfig.ConnectionString
			existingOrg.QueueMappings = queueMappings
			if err := orgService.UpdateOrganization(ctx, existingOrg); err != nil {
				logger.WithError(err).WithField("organization", orgConfig.Name).
					Error("Failed to update organization")
			} else {
				logger.WithField("organization", orgConfig.Name).
					Info("Organization updated successfully")
			}
		} else {
			// Create new organization
			org := &core.Organization{
				Name:             orgConfig.Name,
				ConnectionString: orgConfig.ConnectionString,
				QueueMappings:    queueMappings,
				Active:           true,
				DeviceLimit:      1000, // Default limit
			}
			if err := orgService.CreateOrganization(ctx, org); err != nil {
				logger.WithError(err).WithField("organization", orgConfig.Name).
					Error("Failed to create organization")
			} else {
				logger.WithField("organization", orgConfig.Name).
					Info("Organization created successfully")
			}
		}
	}

	return nil
}

func setupMQTTSubscriber(telemetry *core.TelemetryService, logger *logrus.Logger) (*infrastructure.MQTTSubscriber, error) {
	if cfg.MQTT == nil || cfg.MQTT.BrokerURL == "" {
		logger.Info("MQTT configuration not found, skipping MQTT setup")
		return nil, nil
	}

	mqttConfig := infrastructure.MQTTConfig{
		BrokerURL:    cfg.MQTT.BrokerURL,
		ClientID:     cfg.MQTT.ClientID,
		Username:     cfg.MQTT.Username,
		Password:     cfg.MQTT.Password,
		QoS:          cfg.MQTT.QoS,
		CleanSession: cfg.MQTT.CleanSession,
		Topics:       cfg.MQTT.Topics,
	}

	subscriber, err := infrastructure.NewMQTTSubscriber(mqttConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create MQTT subscriber: %w", err)
	}

	// Register telemetry handler
	subscriber.RegisterHandler("telemetry", func(ctx context.Context, topic string, payload []byte) error {
		// Parse MQTT message to extract device UID and telemetry
		var mqttMsg struct {
			DeviceUID string          `json:"device_uid"`
			Telemetry *core.Telemetry `json:"telemetry"`
		}

		if err := json.Unmarshal(payload, &mqttMsg); err != nil {
			logger.WithError(err).Error("Failed to unmarshal MQTT telemetry")
			return err
		}

		if mqttMsg.DeviceUID == "" || mqttMsg.Telemetry == nil {
			return fmt.Errorf("invalid MQTT telemetry message")
		}

		// Ingest telemetry
		return telemetry.IngestTelemetry(ctx, mqttMsg.DeviceUID, mqttMsg.Telemetry)
	})

	// Start subscriber
	if err := subscriber.Start(); err != nil {
		return nil, fmt.Errorf("failed to start MQTT subscriber: %w", err)
	}

	logger.Info("MQTT subscriber started successfully")
	return subscriber, nil
}
