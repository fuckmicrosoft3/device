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

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"go.novek.io/device/config"
	"go.novek.io/device/internal/infrastructure"
	"go.novek.io/device/internal/utils"
	"gorm.io/gorm"
)

// =============================================================================
// DEVICE MANAGEMENT SERVICE
// =============================================================================

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

	// Set organization name on device
	device.OrganizationName = org.Name

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

func (s *DeviceManagementService) RegisterDeviceBatch(ctx context.Context, requests []BatchRegistrationRequest) (*BatchRegistrationResponse, error) {
	response := &BatchRegistrationResponse{
		Successful: make([]DeviceRegistrationResult, 0),
		Failed:     make([]DeviceRegistrationResult, 0),
	}

	// Validate all devices first
	devices := make([]*Device, 0, len(requests))
	organizations := make(map[uint]*Organization)

	for _, req := range requests {
		device := &Device{
			DeviceUID:       req.DeviceUID,
			OrganizationID:  req.OrganizationID,
			SerialNumber:    req.SerialNumber,
			HardwareVersion: req.HardwareVersion,
			Active:          true,
			UpdatesEnabled:  true,
		}

		if device.DeviceUID == "" {
			device.DeviceUID = uuid.New().String()
		}

		if device.SerialNumber == "" {
			device.SerialNumber = device.DeviceUID
		}

		devices = append(devices, device)
	}

	// Process registration within a transaction
	err := s.store.WithTransaction(ctx, func(ctx context.Context, tx DataStore) error {
		for _, device := range devices {
			// Check if device already exists
			existing, err := tx.GetDeviceByUID(ctx, device.DeviceUID)
			if err == nil && existing != nil {
				response.Failed = append(response.Failed, DeviceRegistrationResult{
					Device: device,
					Error:  "device already exists",
				})
				continue
			}

			// Validate organization (cache the result)
			var org *Organization
			if cachedOrg, exists := organizations[device.OrganizationID]; exists {
				org = cachedOrg
			} else {
				org, err = tx.GetOrganization(ctx, device.OrganizationID)
				if err != nil {
					response.Failed = append(response.Failed, DeviceRegistrationResult{
						Device: device,
						Error:  "organization not found",
					})
					continue
				}
				if !org.Active {
					response.Failed = append(response.Failed, DeviceRegistrationResult{
						Device: device,
						Error:  "organization inactive",
					})
					continue
				}
				organizations[device.OrganizationID] = org
			}

			// Check device limit for this organization
			existingDevices, err := tx.ListDevicesByOrganization(ctx, org.ID)
			if err == nil && len(existingDevices) >= org.DeviceLimit {
				response.Failed = append(response.Failed, DeviceRegistrationResult{
					Device: device,
					Error:  "organization device limit reached",
				})
				continue
			}

			// Set organization name
			device.OrganizationName = org.Name

			// Create device
			if err := tx.CreateDevice(ctx, device); err != nil {
				response.Failed = append(response.Failed, DeviceRegistrationResult{
					Device: device,
					Error:  fmt.Sprintf("failed to create device: %v", err),
				})
				continue
			}

			response.Successful = append(response.Successful, DeviceRegistrationResult{
				Device: device,
			})

			// Cache the device
			s.cacheDevice(ctx, device)
		}

		// If all devices failed, return an error to rollback the transaction
		if len(response.Successful) == 0 {
			return errors.New("all device registrations failed")
		}

		return nil
	})

	if err != nil && len(response.Successful) == 0 {
		return nil, fmt.Errorf("batch registration failed: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"successful": len(response.Successful),
		"failed":     len(response.Failed),
		"total":      len(requests),
	}).Info("Batch device registration completed")

	return response, nil
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
		s.cache.Set(ctx, "device:"+device.DeviceUID, string(data), 24*time.Hour)
	}
}

func (s *DeviceManagementService) getCachedDevice(ctx context.Context, uid string) (*Device, error) {
	if s.cache == nil {
		return nil, errors.New("cache not available")
	}

	data, err := s.cache.Get(ctx, "device:"+uid)
	if err != nil {
		return nil, err
	}

	var device Device
	if err := json.Unmarshal([]byte(data), &device); err != nil {
		return nil, err
	}

	return &device, nil
}

// =============================================================================
// TELEMETRY SERVICE
// =============================================================================

type TelemetryService struct {
	store       DataStore
	messaging   *infrastructure.Messaging
	logger      *logrus.Logger
	processor   *TelemetryProcessor
	queueRouter *QueueRouter
}

func NewTelemetryService(store DataStore, messaging *infrastructure.Messaging, logger *logrus.Logger, cfg config.QueueRoutingConfig) *TelemetryService {
	queueRouter := NewQueueRouter(cfg, logger)
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

func (s *TelemetryService) IngestTelemetry(ctx context.Context, device *Device, telemetryData json.RawMessage) error {
	// Parse telemetry to extract type
	var payload map[string]interface{}
	if err := json.Unmarshal(telemetryData, &payload); err != nil {
		return fmt.Errorf("invalid telemetry format: %w", err)
	}

	telemetryType, ok := payload["ev"].(string)
	if !ok {
		return BusinessError{"TELEMETRY_002", "missing or invalid 'ev' field in telemetry"}
	}

	// Create telemetry record
	telemetry := &Telemetry{
		ID:                 uuid.New().String(),
		DeviceID:           device.ID,
		DeviceUID:          device.DeviceUID,
		DeviceSerialNumber: device.SerialNumber,
		TelemetryType:      telemetryType,
		Payload:            telemetryData,
		ReceivedAt:         time.Now(),
	}

	// Queue for processing
	return s.processor.QueueTelemetry(telemetry)
}

func (s *TelemetryService) GetDeviceTelemetry(ctx context.Context, deviceID uint, limit int) ([]*Telemetry, error) {
	return s.store.GetDeviceTelemetry(ctx, deviceID, limit)
}

func (s *TelemetryService) IngestBatch(ctx context.Context, device *Device, messages []json.RawMessage) error {
	for i, msg := range messages {
		// Ingest each telemetry message for the authenticated device
		if err := s.IngestTelemetry(ctx, device, msg); err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"device_uid":    device.DeviceUID,
				"message_index": i,
			}).Error("Failed to ingest batch telemetry message")
			// Continue processing other messages even if one fails
		}
	}

	return nil
}

func (s *TelemetryService) GetIngestionStats() map[string]interface{} {
	if s.processor != nil {
		return s.processor.Stats()
	}
	return map[string]interface{}{
		"status": "processor not available",
	}
}

func (s *TelemetryService) Stop() {
	if s.processor != nil {
		s.processor.Stop()
	}
}

// =============================================================================
// TELEMETRY PROCESSOR (Supporting structs for TelemetryService)
// =============================================================================

type TelemetryProcessor struct {
	store       DataStore
	messaging   *infrastructure.Messaging
	logger      *logrus.Logger
	queueRouter *QueueRouter
	queue       chan *Telemetry
	retryQueue  chan *RetryItem
	shutdown    chan struct{}
	wg          sync.WaitGroup
	workers     int
	stats       *ProcessorStats
}

type RetryItem struct {
	Telemetry   *Telemetry
	RetryCount  int
	NextRetryAt time.Time
}

type ProcessorStats struct {
	mu        sync.RWMutex
	Processed int64
	Failed    int64
	Retried   int64
}

func NewTelemetryProcessor(store DataStore, messaging *infrastructure.Messaging, logger *logrus.Logger, queueRouter *QueueRouter) *TelemetryProcessor {
	return &TelemetryProcessor{
		store:       store,
		messaging:   messaging,
		logger:      logger,
		queueRouter: queueRouter,
		queue:       make(chan *Telemetry, 1000),
		retryQueue:  make(chan *RetryItem, 500),
		shutdown:    make(chan struct{}),
		stats:       &ProcessorStats{},
	}
}

func (p *TelemetryProcessor) Start(workers int) {
	p.workers = workers
	for i := range workers {
		p.wg.Add(1)
		go p.worker(i)
	}

	p.wg.Add(1)
	go p.retryWorker()

	p.logger.WithField("workers", workers).Info("Telemetry processor started")
}

func (p *TelemetryProcessor) Stop() {
	close(p.shutdown)
	p.wg.Wait()
	p.logger.Info("Telemetry processor stopped")
}

func (p *TelemetryProcessor) QueueTelemetry(telemetry *Telemetry) error {
	select {
	case p.queue <- telemetry:
		return nil
	default:
		// Queue is full, persist for later processing
		if err := p.store.SaveTelemetry(context.Background(), telemetry); err != nil {
			p.logger.WithError(err).Error("Failed to persist telemetry when queue full")
			return err
		}
		return BusinessError{"TELEMETRY_001", "telemetry queue full, message persisted for later processing"}
	}
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

		// Route to appropriate queue
		if p.messaging != nil {
			queueName, err := p.queueRouter.RouteMessage(telemetry)
			if err != nil {
				p.logger.WithError(err).WithField("telemetry_id", telemetry.ID).Error("Failed to route telemetry")
				return err
			}

			if queueName != "" {
				if err := p.messaging.PublishToQueue(ctx, queueName, telemetry, 3); err != nil {
					return fmt.Errorf("failed to send to queue %s: %w", queueName, err)
				}

				// Update published status
				now := time.Now()
				if err := tx.UpdateTelemetryPublishStatus(ctx, telemetry.ID, true, &now, queueName); err != nil {
					return fmt.Errorf("failed to update publish status: %w", err)
				}

				telemetry.Published = true
				telemetry.PublishedAt = &now
				telemetry.PublishedToQueue = queueName
			}
		}

		return nil
	})

	if err != nil {
		p.updateStats(func(s *ProcessorStats) { s.Failed++ })

		// Retry logic
		if retryCount < 3 {
			retryDelay := time.Duration(1<<retryCount) * time.Second
			retryItem := &RetryItem{
				Telemetry:   telemetry,
				RetryCount:  retryCount + 1,
				NextRetryAt: time.Now().Add(retryDelay),
			}

			select {
			case p.retryQueue <- retryItem:
				p.updateStats(func(s *ProcessorStats) { s.Retried++ })
			default:
				p.logger.Error("Retry queue full, dropping retry")
			}
		}

		p.logger.WithError(err).WithField("telemetry_id", telemetry.ID).Error("Failed to process telemetry")
	} else {
		p.updateStats(func(s *ProcessorStats) { s.Processed++ })
	}
}

// =============================================================================
// QUEUE ROUTER (Supporting structs for TelemetryService)
// =============================================================================

type QueueRouter struct {
	config config.QueueRoutingConfig
	logger *logrus.Logger
}

func NewQueueRouter(config config.QueueRoutingConfig, logger *logrus.Logger) *QueueRouter {
	return &QueueRouter{
		config: config,
		logger: logger,
	}
}

func (r *QueueRouter) RouteMessage(telemetry *Telemetry) (string, error) {
	// Find organization config
	var orgConfig *config.QueueConfig
	for _, org := range r.config.Organizations {
		if org.OrganizationName == telemetry.DeviceUID {
			orgConfig = &org
			break
		}
	}

	if orgConfig == nil {
		return "", errors.New("no queue configuration found for organization")
	}

	// Find appropriate queue for telemetry type
	for _, route := range orgConfig.QueueRoutes {
		for _, telType := range route.TelemetryTypes {
			if telType == telemetry.TelemetryType {
				return route.QueueName, nil
			}
		}
	}

	return "", fmt.Errorf("no queue route found for telemetry type: %s", telemetry.TelemetryType)
}

// =============================================================================
// FIRMWARE MANAGEMENT SERVICE
// =============================================================================

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

	// Ensure storage directory exists
	if err := os.MkdirAll(cfg.StoragePath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
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
	if err := os.MkdirAll(filepath.Dir(storagePath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(storagePath, data, 0o644); err != nil {
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
		return BusinessError{
			"FIRMWARE_008",
			fmt.Sprintf("invalid status transition from %s to %s", release.ReleaseStatus, newStatus),
		}
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

// =============================================================================
// UPDATE MANAGEMENT SERVICE
// =============================================================================

type UpdateManagementService struct {
	store           DataStore
	firmwareSvc     *FirmwareManagementService
	logger          *logrus.Logger
	chunkSize       int
	chunkCache      map[string]*cachedChunk
	cacheMu         sync.RWMutex
	processingMutex sync.Mutex
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
			return nil, ErrUpdateSessionNotFound
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

// --- Enhanced OTA Methods ---

func (s *UpdateManagementService) AcknowledgeUpdate(ctx context.Context, sessionID string) error {
	session, err := s.store.GetUpdateSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUpdateSessionNotFound
		}
		return err
	}

	if session.UpdateStatus != UpdateStatusInitiated {
		return BusinessError{
			"UPDATE_009",
			"cannot acknowledge update in status " + session.UpdateStatus,
		}
	}

	session.UpdateStatus = UpdateStatusAcknowledged
	if err := s.store.UpdateUpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"device_id":  session.DeviceID,
	}).Info("Update acknowledged by device")

	return nil
}

func (s *UpdateManagementService) CompleteFlash(ctx context.Context, sessionID string) error {
	return s.store.WithTransaction(ctx, func(ctx context.Context, tx DataStore) error {
		session, err := tx.GetUpdateSession(ctx, sessionID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUpdateSessionNotFound
			}
			return err
		}

		if session.UpdateStatus != UpdateStatusDownloading {
			return BusinessError{
				"UPDATE_010",
				"cannot complete flash in status " + session.UpdateStatus,
			}
		}

		// Get the device and firmware info
		device, err := tx.GetDevice(ctx, session.DeviceID)
		if err != nil {
			return fmt.Errorf("failed to get device: %w", err)
		}

		firmware, err := tx.GetFirmwareRelease(ctx, session.FirmwareID)
		if err != nil {
			return fmt.Errorf("failed to get firmware: %w", err)
		}

		// Update session status
		session.UpdateStatus = UpdateStatusFlashed
		now := time.Now()
		session.CompletedAt = &now
		session.ProgressPercent = 100

		if err := tx.UpdateUpdateSession(ctx, session); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}

		// Update device firmware version
		device.FirmwareVersion = firmware.Version
		if err := tx.UpdateDevice(ctx, device); err != nil {
			return fmt.Errorf("failed to update device firmware version: %w", err)
		}

		s.logger.WithFields(logrus.Fields{
			"session_id":       sessionID,
			"device_id":        session.DeviceID,
			"firmware_version": firmware.Version,
		}).Info("Firmware flash completed")

		return nil
	})
}

// --- Batch Update Methods ---

func (s *UpdateManagementService) CreateUpdateBatch(ctx context.Context, req CreateUpdateBatchRequest) (*UpdateBatch, error) {
	// Validate firmware exists
	firmware, err := s.store.GetFirmwareRelease(ctx, req.FirmwareID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFirmwareNotFound
		}
		return nil, err
	}

	if firmware.ReleaseStatus != ReleaseStatusApproved {
		return nil, ErrFirmwareNotActive
	}

	// Validate devices exist
	validDeviceIDs := make([]uint, 0)
	for _, deviceID := range req.DeviceIDs {
		device, err := s.store.GetDevice(ctx, deviceID)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"device_id": deviceID,
				"error":     err,
			}).Warn("Skipping invalid device in batch")
			continue
		}

		if !device.Active || !device.UpdatesEnabled {
			s.logger.WithFields(logrus.Fields{
				"device_id": deviceID,
				"active":    device.Active,
				"updates":   device.UpdatesEnabled,
			}).Warn("Skipping inactive device or updates disabled")
			continue
		}

		validDeviceIDs = append(validDeviceIDs, deviceID)
	}

	if len(validDeviceIDs) == 0 {
		return nil, BusinessError{"BATCH_001", "no valid devices found for batch update"}
	}

	var batch *UpdateBatch
	err = s.store.WithTransaction(ctx, func(ctx context.Context, tx DataStore) error {
		// Create batch
		batch = &UpdateBatch{
			Name:       req.Name,
			FirmwareID: req.FirmwareID,
			Status:     BatchStatusPending,
		}

		if err := tx.CreateUpdateBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to create update batch: %w", err)
		}

		// Create batch device records
		batchDevices := make([]*UpdateBatchDevice, 0, len(validDeviceIDs))
		for _, deviceID := range validDeviceIDs {
			batchDevice := &UpdateBatchDevice{
				UpdateBatchID: batch.ID,
				DeviceID:      deviceID,
				Status:        BatchDeviceStatusPending,
			}
			batchDevices = append(batchDevices, batchDevice)
		}

		if err := tx.CreateUpdateBatchDevices(ctx, batchDevices); err != nil {
			return fmt.Errorf("failed to create batch devices: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Start background processing
	go s.processBatch(context.Background(), batch.ID)

	s.logger.WithFields(logrus.Fields{
		"batch_id":     batch.ID,
		"batch_name":   batch.Name,
		"firmware_id":  batch.FirmwareID,
		"device_count": len(validDeviceIDs),
	}).Info("Update batch created")

	return s.store.GetUpdateBatch(ctx, batch.ID)
}

func (s *UpdateManagementService) ListUpdateBatches(ctx context.Context) ([]*UpdateBatch, error) {
	return s.store.ListUpdateBatches(ctx)
}

func (s *UpdateManagementService) GetUpdateBatch(ctx context.Context, id uint) (*UpdateBatch, error) {
	batch, err := s.store.GetUpdateBatch(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, BusinessError{"BATCH_002", "update batch not found"}
		}
		return nil, err
	}
	return batch, nil
}

func (s *UpdateManagementService) processBatch(ctx context.Context, batchID uint) {
	s.processingMutex.Lock()
	defer s.processingMutex.Unlock()

	batch, err := s.store.GetUpdateBatch(ctx, batchID)
	if err != nil {
		s.logger.WithError(err).WithField("batch_id", batchID).Error("Failed to get batch for processing")
		return
	}

	// Update batch status to in progress
	batch.Status = BatchStatusInProgress
	if err := s.store.UpdateUpdateBatch(ctx, batch); err != nil {
		s.logger.WithError(err).WithField("batch_id", batchID).Error("Failed to update batch status")
		return
	}

	// Get all batch devices
	batchDevices, err := s.store.GetUpdateBatchDevices(ctx, batchID)
	if err != nil {
		s.logger.WithError(err).WithField("batch_id", batchID).Error("Failed to get batch devices")
		return
	}

	successCount := 0
	failureCount := 0

	// Process each device
	for _, batchDevice := range batchDevices {
		if batchDevice.Status != BatchDeviceStatusPending {
			continue
		}

		// Initiate update for device
		session, err := s.InitiateUpdate(ctx, batchDevice.DeviceID, batch.FirmwareID)
		if err != nil {
			// Mark device as failed
			batchDevice.Status = BatchDeviceStatusFailed
			failureCount++
			s.logger.WithFields(logrus.Fields{
				"batch_id":  batchID,
				"device_id": batchDevice.DeviceID,
				"error":     err,
			}).Error("Failed to initiate update for batch device")
		} else {
			// Mark device as initiated and link to session
			batchDevice.Status = BatchDeviceStatusInitiated
			batchDevice.UpdateSessionID = &session.SessionID
			successCount++
		}

		// Update batch device status
		if err := s.store.UpdateUpdateBatchDevice(ctx, batchDevice); err != nil {
			s.logger.WithError(err).WithField("batch_device_id", batchDevice.ID).Error("Failed to update batch device status")
		}
	}

	// Update overall batch status
	if successCount > 0 {
		batch.Status = BatchStatusInProgress
	} else {
		batch.Status = BatchStatusFailed
	}

	if err := s.store.UpdateUpdateBatch(ctx, batch); err != nil {
		s.logger.WithError(err).WithField("batch_id", batchID).Error("Failed to update final batch status")
	}

	s.logger.WithFields(logrus.Fields{
		"batch_id":      batchID,
		"success_count": successCount,
		"failure_count": failureCount,
		"total_devices": len(batchDevices),
	}).Info("Batch processing completed")
}

// --- Cache Management ---

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

func (s *UpdateManagementService) GetUpdateMetrics() map[string]interface{} {
	s.cacheMu.RLock()
	cacheSize := len(s.chunkCache)
	s.cacheMu.RUnlock()

	return map[string]interface{}{
		"cache_size": cacheSize,
		"timestamp":  time.Now(),
		"chunk_size": s.chunkSize,
	}
}

// =============================================================================
// ORGANIZATION SERVICE
// =============================================================================

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

// =============================================================================
// AUTHENTICATION SERVICE
// =============================================================================

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

func (s *AuthenticationService) ListAccessTokens(ctx context.Context) ([]*AccessToken, error) {
	return nil, errors.New("not implemented")
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
