// services/device/internal/core/service.go
package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"example.com/backstage/services/device/config"
	"example.com/backstage/services/device/internal/infrastructure"
	"example.com/backstage/services/device/internal/utils"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// FirmwareMetadata contains metadata for firmware releases
type FirmwareMetadata struct {
	Filename       string
	Version        string
	ReleaseChannel string
	ReleaseNotes   string
}

// --- Device Management Service Implementation ---

type DeviceManagementService struct {
	store  DataStore
	cache  *infrastructure.Cache
	logger *logrus.Logger
}

func NewDeviceManagementService(store DataStore, cache *infrastructure.Cache, logger *logrus.Logger) *DeviceManagementService {
	return &DeviceManagementService{
		store:  store,
		cache:  cache,
		logger: logger,
	}
}

func (s *DeviceManagementService) RegisterDevice(ctx context.Context, device *Device) error {
	if device.DeviceUID == "" {
		device.DeviceUID = uuid.New().String()
	}

	// Check if device already exists
	existing, err := s.store.GetDeviceByUID(ctx, device.DeviceUID)
	if err == nil && existing != nil {
		return ErrDeviceAlreadyExists
	}

	// Validate organization exists
	org, err := s.store.GetOrganization(ctx, device.OrganizationID)
	if err != nil {
		return ErrOrganizationNotFound
	}
	if !org.Active {
		return ErrOrganizationInactive
	}

	// Check device limit
	devices, err := s.store.ListDevicesByOrganization(ctx, org.ID)
	if err == nil && len(devices) >= org.DeviceLimit {
		return BusinessError{"ORG_003", "organization device limit reached"}
	}

	if err := s.store.CreateDevice(ctx, device); err != nil {
		return fmt.Errorf("failed to register device: %w", err)
	}

	s.cacheDevice(ctx, device)
	s.logger.WithFields(logrus.Fields{
		"device_uid": device.DeviceUID,
		"org_id":     device.OrganizationID,
	}).Info("Device registered successfully")

	return nil
}

func (s *DeviceManagementService) GetDevice(ctx context.Context, id uint) (*Device, error) {
	device, err := s.store.GetDevice(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}
	return device, nil
}

func (s *DeviceManagementService) GetDeviceByUID(ctx context.Context, uid string) (*Device, error) {
	// Try cache first
	if cached, err := s.getCachedDevice(ctx, uid); err == nil && cached != nil {
		return cached, nil
	}

	device, err := s.store.GetDeviceByUID(ctx, uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	s.cacheDevice(ctx, device)
	return device, nil
}

func (s *DeviceManagementService) ListOrganizationDevices(ctx context.Context, orgID uint) ([]*Device, error) {
	return s.store.ListDevicesByOrganization(ctx, orgID)
}

func (s *DeviceManagementService) UpdateDeviceStatus(ctx context.Context, id uint, active bool) error {
	device, err := s.store.GetDevice(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDeviceNotFound
		}
		return err
	}

	device.Active = active
	if err := s.store.UpdateDevice(ctx, device); err != nil {
		return err
	}

	s.cacheDevice(ctx, device)
	return nil
}

func (s *DeviceManagementService) RecordHeartbeat(ctx context.Context, deviceUID string) error {
	device, err := s.store.GetDeviceByUID(ctx, deviceUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDeviceNotFound
		}
		return err
	}

	return s.store.UpdateDeviceHeartbeat(ctx, device.ID)
}

func (s *DeviceManagementService) cacheDevice(ctx context.Context, device *Device) {
	if s.cache != nil {
		data, _ := json.Marshal(device)
		s.cache.Set(ctx, fmt.Sprintf("device:%s", device.DeviceUID), string(data), 24*time.Hour)
	}
}

func (s *DeviceManagementService) getCachedDevice(ctx context.Context, uid string) (*Device, error) {
	if s.cache == nil {
		return nil, errors.New("cache not available")
	}

	data, err := s.cache.Get(ctx, fmt.Sprintf("device:%s", uid))
	if err != nil {
		return nil, err
	}

	var device Device
	if err := json.Unmarshal([]byte(data), &device); err != nil {
		return nil, err
	}

	return &device, nil
}

// --- Telemetry Service Implementation ---

type TelemetryService struct {
	store       DataStore
	messaging   *infrastructure.Messaging
	logger      *logrus.Logger
	processor   *TelemetryProcessor
	queueRouter *QueueRouter
}

func NewTelemetryService(store DataStore, messaging *infrastructure.Messaging, logger *logrus.Logger, cfg config.QueueRoutingConfig) *TelemetryService {
	queueRouter := NewQueueRouter(cfg)
	processor := NewTelemetryProcessor(store, messaging, logger, queueRouter)
	svc := &TelemetryService{
		store:       store,
		messaging:   messaging,
		logger:      logger,
		processor:   processor,
		queueRouter: queueRouter,
	}
	processor.Start(10) // 10 workers
	return svc
}

func (s *TelemetryService) IngestTelemetry(ctx context.Context, device *Device, rawPayload json.RawMessage) error {
	// Parse raw JSON to extract "ev" field
	var payloadMap map[string]interface{}
	if err := json.Unmarshal(rawPayload, &payloadMap); err != nil {
		return BusinessError{"TELEMETRY_002", "invalid JSON payload"}
	}

	// Extract and validate "ev" field
	evValue, exists := payloadMap["ev"]
	if !exists {
		return BusinessError{"TELEMETRY_003", "missing required field 'ev' in payload"}
	}

	telemetryType, ok := evValue.(string)
	if !ok || telemetryType == "" {
		return BusinessError{"TELEMETRY_004", "field 'ev' must be a non-empty string"}
	}

	// Validate device is active
	if !device.Active {
		return ErrDeviceInactive
	}

	// Generate server-side UUID
	telemetryID := uuid.New().String()

	// Add UUID to payload
	payloadMap["uuid"] = telemetryID

	// Re-marshal enriched payload
	enrichedPayload, err := json.Marshal(payloadMap)
	if err != nil {
		return fmt.Errorf("failed to marshal enriched payload: %w", err)
	}

	// Create telemetry record
	telemetry := &Telemetry{
		ID:                 telemetryID,
		DeviceID:           device.ID,
		DeviceUID:          device.DeviceUID,
		DeviceSerialNumber: device.SerialNumber,
		TelemetryType:      telemetryType,
		Payload:            enrichedPayload,
		ReceivedAt:         time.Now(),
		Published:          false,
		PublishedAt:        nil,
		ProcessingError:    false,
	}

	// Determine target queue based on organization and telemetry type
	targetQueue, err := s.queueRouter.GetQueueForTelemetry(device.OrganizationName, telemetryType)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"organization":   device.OrganizationName,
			"telemetry_type": telemetryType,
		}).Warn("No queue route found for telemetry type")
		telemetry.ProcessingError = true
		telemetry.ProcessingErrorMessage = "no queue route configured"
	} else {
		telemetry.PublishedToQueue = targetQueue
	}

	// Enqueue for async processing with reliability
	return s.processor.Enqueue(telemetry)
}

func (s *TelemetryService) IngestBatch(ctx context.Context, device *Device, rawPayloads []json.RawMessage) error {
	var telemetryBatch []*Telemetry

	for _, rawPayload := range rawPayloads {
		// Parse raw JSON to extract "ev" field
		var payloadMap map[string]interface{}
		if err := json.Unmarshal(rawPayload, &payloadMap); err != nil {
			s.logger.WithError(err).Warn("Invalid JSON in batch, skipping")
			continue
		}

		// Extract and validate "ev" field
		evValue, exists := payloadMap["ev"]
		if !exists {
			s.logger.Warn("Missing 'ev' field in batch payload, skipping")
			continue
		}

		telemetryType, ok := evValue.(string)
		if !ok || telemetryType == "" {
			s.logger.Warn("Invalid 'ev' field in batch payload, skipping")
			continue
		}

		// Generate server-side UUID
		telemetryID := uuid.New().String()

		// Add UUID to payload
		payloadMap["uuid"] = telemetryID

		// Re-marshal enriched payload
		enrichedPayload, err := json.Marshal(payloadMap)
		if err != nil {
			s.logger.WithError(err).Warn("Failed to marshal enriched payload, skipping")
			continue
		}

		// Create telemetry record
		telemetry := &Telemetry{
			ID:                 telemetryID,
			DeviceID:           device.ID,
			DeviceUID:          device.DeviceUID,
			DeviceSerialNumber: device.SerialNumber,
			TelemetryType:      telemetryType,
			Payload:            enrichedPayload,
			ReceivedAt:         time.Now(),
			Published:          false,
			PublishedAt:        nil,
			ProcessingError:    false,
		}

		// Determine target queue
		targetQueue, err := s.queueRouter.GetQueueForTelemetry(device.OrganizationName, telemetryType)
		if err != nil {
			telemetry.ProcessingError = true
			telemetry.ProcessingErrorMessage = "no queue route configured"
		} else {
			telemetry.PublishedToQueue = targetQueue
		}

		telemetryBatch = append(telemetryBatch, telemetry)
	}

	// Process valid telemetry
	for _, telemetry := range telemetryBatch {
		if err := s.processor.Enqueue(telemetry); err != nil {
			s.logger.WithError(err).WithField("telemetry_id", telemetry.ID).
				Warn("Failed to enqueue telemetry from batch")
		}
	}

	return nil
}

func (s *TelemetryService) GetDeviceTelemetry(ctx context.Context, deviceID uint, limit int) ([]*Telemetry, error) {
	return s.store.GetDeviceTelemetry(ctx, deviceID, limit)
}

func (s *TelemetryService) GetIngestionStats() map[string]interface{} {
	return s.processor.Stats()
}

func (s *TelemetryService) Stop() {
	s.processor.Stop()
}

// QueueRouter handles routing telemetry to appropriate queues
type QueueRouter struct {
	routes map[string]map[string]string // organization -> telemetryType -> queueName
	logger *logrus.Logger
}

func NewQueueRouter(cfg config.QueueRoutingConfig) *QueueRouter {
	router := &QueueRouter{
		routes: make(map[string]map[string]string),
		logger: logrus.New(),
	}

	// Build routing table from configuration
	for _, orgConfig := range cfg.Organizations {
		orgRoutes := make(map[string]string)
		for _, queueRoute := range orgConfig.QueueRoutes {
			for _, telemetryType := range queueRoute.TelemetryTypes {
				orgRoutes[telemetryType] = queueRoute.QueueName
			}
		}
		router.routes[orgConfig.OrganizationName] = orgRoutes
	}

	return router
}

func (r *QueueRouter) GetQueueForTelemetry(organizationName, telemetryType string) (string, error) {
	orgRoutes, exists := r.routes[organizationName]
	if !exists {
		return "", fmt.Errorf("no routes configured for organization: %s", organizationName)
	}

	queueName, exists := orgRoutes[telemetryType]
	if !exists {
		return "", fmt.Errorf("no queue configured for telemetry type: %s", telemetryType)
	}

	return queueName, nil
}

// TelemetryProcessor handles async telemetry processing with reliability
type TelemetryProcessor struct {
	store         DataStore
	messaging     *infrastructure.Messaging
	logger        *logrus.Logger
	queueRouter   *QueueRouter
	queue         chan *Telemetry
	retryQueue    chan *RetryItem
	persistentWAL *infrastructure.WAL
	workers       int
	wg            sync.WaitGroup
	shutdown      chan struct{}
	stats         *ProcessorStats
}

type RetryItem struct {
	Telemetry   *Telemetry
	RetryCount  int
	LastError   error
	NextRetryAt time.Time
}

type ProcessorStats struct {
	mu              sync.RWMutex
	Processed       uint64
	Failed          uint64
	Retried         uint64
	QueueDepth      int
	RetryQueueDepth int
}

func NewTelemetryProcessor(store DataStore, messaging *infrastructure.Messaging, logger *logrus.Logger, queueRouter *QueueRouter) *TelemetryProcessor {
	// Initialize Write-Ahead Log for persistence
	wal, err := infrastructure.NewWAL("/data/telemetry-wal")
	if err != nil {
		logger.WithError(err).Error("Failed to initialize WAL, using in-memory fallback")
	}

	return &TelemetryProcessor{
		store:         store,
		messaging:     messaging,
		logger:        logger,
		queueRouter:   queueRouter,
		queue:         make(chan *Telemetry, 10000),
		retryQueue:    make(chan *RetryItem, 5000),
		persistentWAL: wal,
		shutdown:      make(chan struct{}),
		stats:         &ProcessorStats{},
	}
}

func (p *TelemetryProcessor) Start(workers int) {
	p.workers = workers

	// Start main workers
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	// Start retry worker
	p.wg.Add(1)
	go p.retryWorker()

	// Start WAL recovery
	if p.persistentWAL != nil {
		p.wg.Add(1)
		go p.recoverFromWAL()
	}

	p.logger.Infof("Started %d telemetry processor workers", workers)
}

func (p *TelemetryProcessor) Stop() {
	close(p.shutdown)
	p.wg.Wait()
	if p.persistentWAL != nil {
		p.persistentWAL.Close()
	}
}

func (p *TelemetryProcessor) Enqueue(telemetry *Telemetry) error {
	// Write to WAL first for durability
	if p.persistentWAL != nil {
		if err := p.persistentWAL.Write(telemetry); err != nil {
			p.logger.WithError(err).Warn("Failed to write to WAL")
		}
	}

	select {
	case p.queue <- telemetry:
		return nil
	default:
		// Queue full, try to persist for later processing
		return p.persistToFallback(telemetry)
	}
}

func (p *TelemetryProcessor) persistToFallback(telemetry *Telemetry) error {
	// Implement fallback persistence (e.g., local file, secondary queue)
	p.logger.Warn("Main queue full, persisting to fallback")
	p.updateStats(func(s *ProcessorStats) {
		s.Failed++
	})
	return BusinessError{"TELEMETRY_001", "telemetry queue full, message persisted for later processing"}
}

func (p *TelemetryProcessor) Stats() map[string]interface{} {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()

	return map[string]interface{}{
		"processed":         p.stats.Processed,
		"failed":            p.stats.Failed,
		"retried":           p.stats.Retried,
		"queue_depth":       len(p.queue),
		"retry_queue_depth": len(p.retryQueue),
		"queue_capacity":    cap(p.queue),
		"workers":           p.workers,
	}
}

func (p *TelemetryProcessor) updateStats(fn func(*ProcessorStats)) {
	p.stats.mu.Lock()
	defer p.stats.mu.Unlock()
	fn(p.stats)
}

func (p *TelemetryProcessor) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.shutdown:
			return
		case telemetry := <-p.queue:
			p.processTelemetry(telemetry, 0)
		}
	}
}

func (p *TelemetryProcessor) retryWorker() {
	defer p.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.shutdown:
			return
		case <-ticker.C:
			p.processRetries()
		case item := <-p.retryQueue:
			if time.Now().After(item.NextRetryAt) {
				p.processTelemetry(item.Telemetry, item.RetryCount)
			} else {
				// Re-queue for later
				select {
				case p.retryQueue <- item:
				default:
					p.logger.Warn("Retry queue full, dropping retry item")
				}
			}
		}
	}
}

func (p *TelemetryProcessor) processRetries() {
	retryItems := make([]*RetryItem, 0)

	// Drain retry queue
	for {
		select {
		case item := <-p.retryQueue:
			retryItems = append(retryItems, item)
		default:
			goto process
		}
	}

process:
	now := time.Now()
	for _, item := range retryItems {
		if now.After(item.NextRetryAt) {
			p.processTelemetry(item.Telemetry, item.RetryCount)
		} else {
			// Re-queue for later
			select {
			case p.retryQueue <- item:
			default:
				p.logger.Warn("Retry queue full during reprocessing")
			}
		}
	}
}

func (p *TelemetryProcessor) processTelemetry(telemetry *Telemetry, retryCount int) {
	ctx := context.Background()

	err := p.store.WithTransaction(ctx, func(ctx context.Context, tx DataStore) error {
		// Save telemetry
		if err := tx.SaveTelemetry(ctx, telemetry); err != nil {
			return fmt.Errorf("failed to save telemetry: %w", err)
		}

		// Update device heartbeat
		if err := tx.UpdateDeviceHeartbeat(ctx, telemetry.DeviceID); err != nil {
			return fmt.Errorf("failed to update heartbeat: %w", err)
		}

		// Publish to messaging system if configured and no processing error
		if p.messaging != nil && !telemetry.ProcessingError && telemetry.PublishedToQueue != "" {
			// Publish to the specific queue determined by router
			if err := p.messaging.PublishToQueue(ctx, telemetry.PublishedToQueue, telemetry, 3); err != nil {
				// Log but don't fail the transaction
				p.logger.WithError(err).WithFields(logrus.Fields{
					"queue":          telemetry.PublishedToQueue,
					"telemetry_type": telemetry.TelemetryType,
					"telemetry_id":   telemetry.ID,
				}).Warn("Failed to publish to messaging system")
			} else {
				// Mark as published
				now := time.Now()
				telemetry.Published = true
				telemetry.PublishedAt = &now
				if err := tx.UpdateTelemetryPublishStatus(ctx, telemetry.ID, true, &now, telemetry.PublishedToQueue); err != nil {
					return fmt.Errorf("failed to update publish status: %w", err)
				}
			}
		}

		return nil
	})

	if err != nil {
		p.handleProcessingError(telemetry, err, retryCount)
	} else {
		// Remove from WAL on success
		if p.persistentWAL != nil {
			p.persistentWAL.Remove(telemetry.ID)
		}
		p.updateStats(func(s *ProcessorStats) {
			s.Processed++
		})

		p.logger.WithFields(logrus.Fields{
			"telemetry_id":   telemetry.ID,
			"telemetry_type": telemetry.TelemetryType,
			"queue":          telemetry.PublishedToQueue,
			"published":      telemetry.Published,
		}).Debug("Telemetry processed successfully")
	}
}

func (p *TelemetryProcessor) handleProcessingError(telemetry *Telemetry, err error, retryCount int) {
	p.logger.WithError(err).WithFields(logrus.Fields{
		"message_id":  telemetry.ID,
		"retry_count": retryCount,
	}).Error("Failed to process telemetry")

	if retryCount < 5 {
		// Exponential backoff
		nextRetry := time.Now().Add(time.Duration(1<<uint(retryCount)) * time.Second)
		retryItem := &RetryItem{
			Telemetry:   telemetry,
			RetryCount:  retryCount + 1,
			LastError:   err,
			NextRetryAt: nextRetry,
		}

		select {
		case p.retryQueue <- retryItem:
			p.updateStats(func(s *ProcessorStats) {
				s.Retried++
			})
		default:
			// Retry queue full, persist to dead letter
			p.persistToDeadLetter(telemetry, err)
		}
	} else {
		// Max retries exceeded, move to dead letter
		p.persistToDeadLetter(telemetry, err)
	}
}

func (p *TelemetryProcessor) persistToDeadLetter(telemetry *Telemetry, err error) {
	// Implement dead letter persistence
	p.logger.WithError(err).WithField("message_id", telemetry.ID).
		Error("Moving telemetry to dead letter queue")

	p.updateStats(func(s *ProcessorStats) {
		s.Failed++
	})

	// Could write to a separate table or file
	// For now, just ensure it's in the WAL for manual recovery
}

func (p *TelemetryProcessor) recoverFromWAL() {
	defer p.wg.Done()

	if p.persistentWAL == nil {
		return
	}

	messages, err := p.persistentWAL.ReadAll()
	if err != nil {
		p.logger.WithError(err).Error("Failed to recover from WAL")
		return
	}

	p.logger.Infof("Recovering %d messages from WAL", len(messages))

	for _, msg := range messages {
		telemetry, ok := msg.(*Telemetry)
		if !ok {
			continue
		}

		select {
		case p.queue <- telemetry:
		default:
			p.logger.Warn("Queue full during WAL recovery")
		}
	}
}

// --- Firmware Management Service Implementation ---

type FirmwareManagementService struct {
	store       DataStore
	logger      *logrus.Logger
	storagePath string
	maxFileSize int64
	signingKey  *utils.SigningKey
}

func NewFirmwareManagementService(store DataStore, logger *logrus.Logger, cfg config.FirmwareConfig) (*FirmwareManagementService, error) {
	svc := &FirmwareManagementService{
		store:       store,
		logger:      logger,
		storagePath: cfg.StoragePath,
		maxFileSize: cfg.MaxFileSize,
	}

	if cfg.SigningEnabled && cfg.PrivateKeyPath != "" {
		key, err := utils.LoadSigningKey(cfg.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load signing key: %w", err)
		}
		svc.signingKey = key
	}

	// Ensure firmware storage exists
	if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create firmware storage: %w", err)
	}

	return svc, nil
}

func (s *FirmwareManagementService) CreateRelease(ctx context.Context, data []byte, metadata FirmwareMetadata) (*FirmwareRelease, error) {
	// Validate version format
	if err := utils.ValidateVersion(metadata.Version); err != nil {
		return nil, BusinessError{"FIRMWARE_005", fmt.Sprintf("invalid version format: %v", err)}
	}

	// Check file size
	if int64(len(data)) > s.maxFileSize {
		return nil, BusinessError{"FIRMWARE_006", fmt.Sprintf("file size exceeds limit of %d bytes", s.maxFileSize)}
	}

	// Check if version already exists
	existing, err := s.store.GetFirmwareByVersion(ctx, metadata.Version)
	if err == nil && existing != nil {
		return nil, BusinessError{"FIRMWARE_007", "version already exists"}
	}

	// Calculate checksum
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])

	// Generate storage path
	timestamp := time.Now().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s_%s_%s",
		metadata.ReleaseChannel, metadata.Version, timestamp, metadata.Filename)
	storagePath := filepath.Join(s.storagePath, metadata.ReleaseChannel, filename)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(storagePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(storagePath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to save firmware: %w", err)
	}

	release := &FirmwareRelease{
		Version:        metadata.Version,
		ReleaseChannel: metadata.ReleaseChannel,
		StoragePath:    storagePath,
		Checksum:       checksum,
		SizeBytes:      int64(len(data)),
		ReleaseStatus:  ReleaseStatusDraft,
		ReleaseNotes:   metadata.ReleaseNotes,
	}

	// Sign if enabled
	if s.signingKey != nil {
		signature, err := s.signingKey.Sign(data)
		if err != nil {
			os.Remove(storagePath) // Clean up
			return nil, fmt.Errorf("failed to sign firmware: %w", err)
		}
		release.DigitalSignature = signature
	}

	if err := s.store.CreateFirmwareRelease(ctx, release); err != nil {
		os.Remove(storagePath) // Clean up
		return nil, fmt.Errorf("failed to create release record: %w", err)
	}

	// Start background validation
	go s.validateRelease(context.Background(), release)

	s.logger.WithFields(logrus.Fields{
		"version": release.Version,
		"channel": release.ReleaseChannel,
		"size":    release.SizeBytes,
	}).Info("Firmware release created")

	return release, nil
}

func (s *FirmwareManagementService) validateRelease(ctx context.Context, release *FirmwareRelease) {
	// Read file and verify checksum
	data, err := os.ReadFile(release.StoragePath)
	if err != nil {
		s.logger.WithError(err).Error("Failed to read firmware for validation")
		return
	}

	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) == release.Checksum {
		release.ReleaseStatus = ReleaseStatusTesting
		if err := s.store.UpdateFirmwareRelease(ctx, release); err != nil {
			s.logger.WithError(err).Error("Failed to update release status")
		}
	}
}

func (s *FirmwareManagementService) GetRelease(ctx context.Context, id uint) (*FirmwareRelease, error) {
	release, err := s.store.GetFirmwareRelease(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFirmwareNotFound
		}
		return nil, err
	}
	return release, nil
}

func (s *FirmwareManagementService) ListReleases(ctx context.Context, channel string) ([]*FirmwareRelease, error) {
	return s.store.ListFirmwareReleases(ctx, channel)
}

func (s *FirmwareManagementService) PromoteRelease(ctx context.Context, id uint, newStatus string) error {
	release, err := s.store.GetFirmwareRelease(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFirmwareNotFound
		}
		return err
	}

	// Validate status transition
	validTransitions := map[string][]string{
		ReleaseStatusDraft:    {ReleaseStatusTesting},
		ReleaseStatusTesting:  {ReleaseStatusApproved, ReleaseStatusDraft},
		ReleaseStatusApproved: {ReleaseStatusDeprecated},
	}

	allowed := false
	for _, status := range validTransitions[release.ReleaseStatus] {
		if status == newStatus {
			allowed = true
			break
		}
	}

	if !allowed {
		return BusinessError{"FIRMWARE_008",
			fmt.Sprintf("invalid status transition from %s to %s", release.ReleaseStatus, newStatus)}
	}

	release.ReleaseStatus = newStatus
	return s.store.UpdateFirmwareRelease(ctx, release)
}

func (s *FirmwareManagementService) GetLatestRelease(ctx context.Context, channel string) (*FirmwareRelease, error) {
	release, err := s.store.GetLatestApprovedFirmware(ctx, channel)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFirmwareNotFound
		}
		return nil, err
	}
	return release, nil
}

// --- Update Management Service Implementation ---

type UpdateManagementService struct {
	store       DataStore
	firmwareSvc *FirmwareManagementService
	logger      *logrus.Logger
	chunkSize   int
	chunkCache  map[string]*cachedChunk
	cacheMu     sync.RWMutex
}

type cachedChunk struct {
	data       []byte
	lastAccess time.Time
}

func NewUpdateManagementService(store DataStore, firmwareSvc *FirmwareManagementService, logger *logrus.Logger, cfg config.OTAConfig) *UpdateManagementService {
	svc := &UpdateManagementService{
		store:       store,
		firmwareSvc: firmwareSvc,
		logger:      logger,
		chunkSize:   cfg.ChunkSize,
		chunkCache:  make(map[string]*cachedChunk),
	}
	go svc.cleanupCache()
	return svc
}

func (s *UpdateManagementService) InitiateUpdate(ctx context.Context, deviceID, firmwareID uint) (*UpdateSession, error) {
	// Validate device
	device, err := s.store.GetDevice(ctx, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	if !device.Active {
		return nil, ErrDeviceInactive
	}

	if !device.UpdatesEnabled {
		return nil, ErrDeviceUpdatesDenied
	}

	// Check for existing active update
	existing, err := s.store.GetActiveDeviceUpdate(ctx, deviceID)
	if err == nil && existing != nil {
		return nil, ErrUpdateInProgress
	}

	// Validate firmware
	firmware, err := s.firmwareSvc.GetRelease(ctx, firmwareID)
	if err != nil {
		return nil, err
	}

	if firmware.ReleaseStatus != ReleaseStatusApproved {
		return nil, ErrFirmwareNotActive
	}

	// Create update session
	session := &UpdateSession{
		SessionID:    uuid.New().String(),
		DeviceID:     deviceID,
		FirmwareID:   firmwareID,
		UpdateStatus: UpdateStatusInitiated,
	}

	if err := s.store.CreateUpdateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create update session: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"session_id":  session.SessionID,
		"device_id":   deviceID,
		"firmware_id": firmwareID,
	}).Info("Update session initiated")

	return s.store.GetUpdateSession(ctx, session.SessionID)
}

func (s *UpdateManagementService) CheckForUpdates(ctx context.Context, deviceUID, currentVersion string) (*UpdateSession, error) {
	device, err := s.store.GetDeviceByUID(ctx, deviceUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	if !device.Active || !device.UpdatesEnabled {
		return nil, nil // No updates available
	}

	// Check for active update
	existing, err := s.store.GetActiveDeviceUpdate(ctx, device.ID)
	if err == nil && existing != nil {
		return existing, nil
	}

	// Get latest firmware for device's channel
	latest, err := s.firmwareSvc.GetLatestRelease(ctx, ReleaseChannelProduction)
	if err != nil {
		return nil, nil // No updates available
	}

	// Compare versions
	if utils.CompareVersions(currentVersion, latest.Version) >= 0 {
		return nil, nil // Already up to date
	}

	// Check minimum version requirement
	if latest.MinimumVersion != "" && utils.CompareVersions(currentVersion, latest.MinimumVersion) < 0 {
		return nil, ErrVersionDowngrade
	}

	// Create update session
	return s.InitiateUpdate(ctx, device.ID, latest.ID)
}

func (s *UpdateManagementService) GetUpdateSession(ctx context.Context, sessionID string) (*UpdateSession, error) {
	session, err := s.store.GetUpdateSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, BusinessError{"OTA_005", "update session not found"}
		}
		return nil, err
	}
	return session, nil
}

func (s *UpdateManagementService) DownloadFirmwareChunk(ctx context.Context, sessionID string, offset, size int64) ([]byte, error) {
	session, err := s.GetUpdateSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Update status if needed
	if session.UpdateStatus == UpdateStatusInitiated {
		session.UpdateStatus = UpdateStatusDownloading
		now := time.Now()
		session.StartedAt = &now
		s.store.UpdateUpdateSession(ctx, session)
	} else if session.UpdateStatus != UpdateStatusDownloading {
		return nil, BusinessError{"OTA_006", "update not in downloadable state"}
	}

	// Check cache
	cacheKey := fmt.Sprintf("%s:%d:%d", sessionID, offset, size)
	if chunk := s.getCachedChunk(cacheKey); chunk != nil {
		return chunk, nil
	}

	// Read firmware file
	file, err := os.Open(session.Firmware.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open firmware: %w", err)
	}
	defer file.Close()

	// Seek and read
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek: %w", err)
	}

	chunk := make([]byte, size)
	n, err := io.ReadFull(file, chunk)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("failed to read chunk: %w", err)
	}
	chunk = chunk[:n]

	// Update progress
	session.BytesTransferred = offset + int64(n)
	session.ProgressPercent = int(float64(session.BytesTransferred) / float64(session.Firmware.SizeBytes) * 100)

	// Calculate transfer rate
	if session.StartedAt != nil {
		elapsed := time.Since(*session.StartedAt).Seconds()
		if elapsed > 0 {
			session.TransferRate = int64(float64(session.BytesTransferred) / elapsed)
		}
	}

	s.store.UpdateUpdateSession(ctx, session)

	// Cache chunk
	s.cacheChunk(cacheKey, chunk)

	return chunk, nil
}

func (s *UpdateManagementService) CompleteUpdate(ctx context.Context, sessionID string, checksum string) error {
	session, err := s.GetUpdateSession(ctx, sessionID)
	if err != nil {
		return err
	}

	if session.UpdateStatus != UpdateStatusDownloading {
		return BusinessError{"OTA_007", "invalid update state for completion"}
	}

	// Verify checksum
	if checksum != session.Firmware.Checksum {
		session.UpdateStatus = UpdateStatusFailed
		session.FailureReason = "checksum verification failed"
		s.store.UpdateUpdateSession(ctx, session)
		return ErrChecksumMismatch
	}

	// Mark as completed
	session.UpdateStatus = UpdateStatusCompleted
	now := time.Now()
	session.CompletedAt = &now
	session.ProgressPercent = 100

	if err := s.store.UpdateUpdateSession(ctx, session); err != nil {
		return err
	}

	// Update device firmware version
	device, err := s.store.GetDevice(ctx, session.DeviceID)
	if err == nil {
		device.FirmwareVersion = session.Firmware.Version
		s.store.UpdateDevice(ctx, device)
	}

	s.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"device_id":  session.DeviceID,
		"version":    session.Firmware.Version,
	}).Info("Update completed successfully")

	return nil
}

func (s *UpdateManagementService) GetUpdateMetrics() map[string]interface{} {
	// This would aggregate metrics from the database
	return map[string]interface{}{
		"cache_size": len(s.chunkCache),
		"timestamp":  time.Now(),
	}
}

func (s *UpdateManagementService) getCachedChunk(key string) []byte {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	if chunk, ok := s.chunkCache[key]; ok {
		chunk.lastAccess = time.Now()
		return chunk.data
	}
	return nil
}

func (s *UpdateManagementService) cacheChunk(key string, data []byte) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.chunkCache[key] = &cachedChunk{
		data:       data,
		lastAccess: time.Now(),
	}
}

func (s *UpdateManagementService) cleanupCache() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cacheMu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for key, chunk := range s.chunkCache {
			if chunk.lastAccess.Before(cutoff) {
				delete(s.chunkCache, key)
			}
		}
		s.cacheMu.Unlock()
	}
}

// --- Organization Service Implementation ---

type OrganizationService struct {
	store  DataStore
	logger *logrus.Logger
}

func NewOrganizationService(store DataStore, logger *logrus.Logger) *OrganizationService {
	return &OrganizationService{
		store:  store,
		logger: logger,
	}
}

func (s *OrganizationService) CreateOrganization(ctx context.Context, org *Organization) error {
	if org.Name == "" {
		return BusinessError{"ORG_004", "organization name is required"}
	}

	if err := s.store.CreateOrganization(ctx, org); err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}

	s.logger.WithField("org_id", org.ID).Info("Organization created")
	return nil
}

func (s *OrganizationService) GetOrganization(ctx context.Context, id uint) (*Organization, error) {
	org, err := s.store.GetOrganization(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}
	return org, nil
}

func (s *OrganizationService) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	return s.store.ListOrganizations(ctx)
}

func (s *OrganizationService) UpdateOrganization(ctx context.Context, org *Organization) error {
	existing, err := s.store.GetOrganization(ctx, org.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrOrganizationNotFound
		}
		return err
	}

	// Preserve immutable fields
	org.CreatedAt = existing.CreatedAt

	return s.store.UpdateOrganization(ctx, org)
}

// --- Authentication Service Implementation ---

type AuthenticationService struct {
	store  DataStore
	logger *logrus.Logger
}

func NewAuthenticationService(store DataStore, logger *logrus.Logger) *AuthenticationService {
	return &AuthenticationService{
		store:  store,
		logger: logger,
	}
}

func (s *AuthenticationService) ValidateToken(ctx context.Context, token string) (*AccessToken, error) {
	accessToken, err := s.store.GetAccessToken(ctx, token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, BusinessError{"AUTH_001", "invalid token"}
		}
		return nil, err
	}

	// Check expiration
	if accessToken.ExpiresAt != nil && accessToken.ExpiresAt.Before(time.Now()) {
		return nil, BusinessError{"AUTH_002", "token expired"}
	}

	// Update last access
	go s.store.UpdateTokenLastAccess(context.Background(), token)

	return accessToken, nil
}

func (s *AuthenticationService) CreateToken(ctx context.Context, description string, scopes []string, expiresIn time.Duration) (*AccessToken, error) {
	// Generate secure token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	token := hex.EncodeToString(tokenBytes)

	var expiresAt *time.Time
	if expiresIn > 0 {
		exp := time.Now().Add(expiresIn)
		expiresAt = &exp
	}

	accessToken := &AccessToken{
		Token:       token,
		Description: description,
		Scopes:      scopes,
		ExpiresAt:   expiresAt,
	}

	if err := s.store.CreateAccessToken(ctx, accessToken); err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	s.logger.WithField("token_id", accessToken.ID).Info("Access token created")
	return accessToken, nil
}

func (s *AuthenticationService) RevokeToken(ctx context.Context, token string) error {
	return s.store.DeleteAccessToken(ctx, token)
}

func (s *AuthenticationService) HasScope(token *AccessToken, scope string) bool {
	for _, s := range token.Scopes {
		if s == scope || s == "admin" {
			return true
		}
	}
	return false
}
