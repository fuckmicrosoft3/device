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
	TestDeviceUID  string // Device to test firmware on
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
		"device_uid":     device.DeviceUID,
		"org_id":         device.OrganizationID,
		"is_test_device": device.IsTestDevice,
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
	store     DataStore
	messaging *infrastructure.Messaging
	logger    *logrus.Logger
	processor *TelemetryProcessor
}

func NewTelemetryService(store DataStore, messaging *infrastructure.Messaging, logger *logrus.Logger) *TelemetryService {
	processor := NewTelemetryProcessor(store, messaging, logger)
	svc := &TelemetryService{
		store:     store,
		messaging: messaging,
		logger:    logger,
		processor: processor,
	}
	processor.Start(10) // 10 workers
	return svc
}

func (s *TelemetryService) IngestTelemetry(ctx context.Context, deviceUID string, telemetry *Telemetry) error {
	if telemetry.MessageID == "" {
		telemetry.MessageID = uuid.New().String()
	}

	// Validate device exists
	device, err := s.store.GetDeviceByUID(ctx, deviceUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDeviceNotFound
		}
		return err
	}

	if !device.Active {
		return ErrDeviceInactive
	}

	// Validate telemetry type
	if telemetry.TelemetryType == "" {
		return ErrInvalidTelemetryType
	}

	telemetry.DeviceID = device.ID
	telemetry.ReceivedAt = time.Now()

	// Add message ID to payload for idempotency
	var payload map[string]interface{}
	if err := json.Unmarshal(telemetry.Payload, &payload); err != nil {
		payload = make(map[string]interface{})
	}
	payload["message_id"] = telemetry.MessageID
	telemetry.Payload, _ = json.Marshal(payload)

	// Enqueue for async processing
	return s.processor.Enqueue(telemetry, device.OrganizationID)
}

func (s *TelemetryService) IngestBatch(ctx context.Context, batch []*Telemetry) error {
	for _, telemetry := range batch {
		if telemetry.MessageID == "" {
			telemetry.MessageID = uuid.New().String()
		}

		// Get device to find organization
		device, err := s.store.GetDevice(ctx, telemetry.DeviceID)
		if err != nil {
			s.logger.WithError(err).WithField("device_id", telemetry.DeviceID).
				Warn("Failed to get device for telemetry")
			continue
		}

		telemetry.ReceivedAt = time.Now()

		if err := s.processor.Enqueue(telemetry, device.OrganizationID); err != nil {
			s.logger.WithError(err).Warn("Failed to enqueue telemetry")
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

// TelemetryProcessor handles async telemetry processing
type TelemetryProcessor struct {
	store     DataStore
	messaging *infrastructure.Messaging
	logger    *logrus.Logger
	queue     chan *TelemetryItem
	workers   int
	wg        sync.WaitGroup
	shutdown  chan struct{}
	stats     *ProcessorStats
	orgCache  map[uint]*Organization
	cacheMu   sync.RWMutex
}

type TelemetryItem struct {
	Telemetry      *Telemetry
	OrganizationID uint
}

type ProcessorStats struct {
	mu         sync.RWMutex
	Processed  uint64
	Failed     uint64
	QueueDepth int
}

func NewTelemetryProcessor(store DataStore, messaging *infrastructure.Messaging, logger *logrus.Logger) *TelemetryProcessor {
	return &TelemetryProcessor{
		store:     store,
		messaging: messaging,
		logger:    logger,
		queue:     make(chan *TelemetryItem, 10000),
		shutdown:  make(chan struct{}),
		stats:     &ProcessorStats{},
		orgCache:  make(map[uint]*Organization),
	}
}

func (p *TelemetryProcessor) Start(workers int) {
	p.workers = workers

	// Start workers
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	p.logger.Infof("Started %d telemetry processor workers", workers)
}

func (p *TelemetryProcessor) Stop() {
	close(p.shutdown)
	p.wg.Wait()
}

func (p *TelemetryProcessor) Enqueue(telemetry *Telemetry, orgID uint) error {
	item := &TelemetryItem{
		Telemetry:      telemetry,
		OrganizationID: orgID,
	}

	select {
	case p.queue <- item:
		return nil
	default:
		p.updateStats(func(s *ProcessorStats) {
			s.Failed++
		})
		return BusinessError{"TELEMETRY_003", "telemetry queue full"}
	}
}

func (p *TelemetryProcessor) Stats() map[string]interface{} {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()

	return map[string]interface{}{
		"processed":      p.stats.Processed,
		"failed":         p.stats.Failed,
		"queue_depth":    len(p.queue),
		"queue_capacity": cap(p.queue),
		"workers":        p.workers,
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
		case item := <-p.queue:
			p.processTelemetry(item)
		}
	}
}

func (p *TelemetryProcessor) processTelemetry(item *TelemetryItem) {
	ctx := context.Background()

	// Get organization with queue mappings
	org, err := p.getOrganization(ctx, item.OrganizationID)
	if err != nil {
		p.logger.WithError(err).WithField("org_id", item.OrganizationID).
			Error("Failed to get organization")
		p.updateStats(func(s *ProcessorStats) {
			s.Failed++
		})
		return
	}

	// Determine target queue
	queueName, err := org.QueueMappings.GetQueueForTelemetryType(item.Telemetry.TelemetryType)
	if err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"telemetry_type": item.Telemetry.TelemetryType,
			"org_id":         item.OrganizationID,
		}).Error("No queue configured for telemetry type")
		item.Telemetry.LastError = err.Error()
	} else {
		item.Telemetry.QueueName = queueName
	}

	err = p.store.WithTransaction(ctx, func(ctx context.Context, tx DataStore) error {
		// Save telemetry
		if err := tx.SaveTelemetry(ctx, item.Telemetry); err != nil {
			return fmt.Errorf("failed to save telemetry: %w", err)
		}

		// Update device heartbeat
		if err := tx.UpdateDeviceHeartbeat(ctx, item.Telemetry.DeviceID); err != nil {
			return fmt.Errorf("failed to update heartbeat: %w", err)
		}

		// Publish to messaging system if queue is configured
		if p.messaging != nil && queueName != "" {
			msgErr := p.messaging.PublishToQueue(ctx, org.ConnectionString, queueName, item.Telemetry)
			if msgErr != nil {
				// Log but don't fail the transaction
				p.logger.WithError(msgErr).WithFields(logrus.Fields{
					"queue":      queueName,
					"message_id": item.Telemetry.MessageID,
				}).Error("Failed to publish to queue")
				item.Telemetry.LastError = msgErr.Error()
			} else {
				if err := tx.MarkTelemetryProcessed(ctx, item.Telemetry.MessageID); err != nil {
					return fmt.Errorf("failed to mark as processed: %w", err)
				}
			}
		}

		return nil
	})

	if err != nil {
		p.logger.WithError(err).WithField("message_id", item.Telemetry.MessageID).
			Error("Failed to process telemetry")
		p.updateStats(func(s *ProcessorStats) {
			s.Failed++
		})
	} else {
		p.updateStats(func(s *ProcessorStats) {
			s.Processed++
		})
	}
}

func (p *TelemetryProcessor) getOrganization(ctx context.Context, orgID uint) (*Organization, error) {
	// Check cache first
	p.cacheMu.RLock()
	org, exists := p.orgCache[orgID]
	p.cacheMu.RUnlock()

	if exists {
		return org, nil
	}

	// Load from database
	org, err := p.store.GetOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// Update cache
	p.cacheMu.Lock()
	p.orgCache[orgID] = org
	p.cacheMu.Unlock()

	return org, nil
}

// --- Firmware Management Service Implementation ---

type FirmwareManagementService struct {
	store       DataStore
	updateMgmt  *UpdateManagementService
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

// SetUpdateManagementService sets the update management service (to avoid circular dependency)
func (s *FirmwareManagementService) SetUpdateManagementService(updateMgmt *UpdateManagementService) {
	s.updateMgmt = updateMgmt
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

	// Validate test device exists if specified
	if metadata.TestDeviceUID != "" {
		testDevice, err := s.store.GetDeviceByUID(ctx, metadata.TestDeviceUID)
		if err != nil || !testDevice.Active || !testDevice.IsTestDevice {
			return nil, ErrTestDeviceNotFound
		}
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

	// Schedule test if test device was specified
	if metadata.TestDeviceUID != "" {
		go s.scheduleTest(context.Background(), release, metadata.TestDeviceUID)
	}

	s.logger.WithFields(logrus.Fields{
		"version": release.Version,
		"channel": release.ReleaseChannel,
		"size":    release.SizeBytes,
	}).Info("Firmware release created")

	return release, nil
}

func (s *FirmwareManagementService) scheduleTest(ctx context.Context, release *FirmwareRelease, testDeviceUID string) {
	// Check if test already exists
	existingTests, err := s.store.GetFirmwareTestResults(ctx, release.ID)
	if err == nil {
		for _, test := range existingTests {
			if test.TestStatus == FirmwareTestPending || test.TestStatus == FirmwareTestRunning {
				s.logger.WithField("firmware_id", release.ID).
					Info("Test already scheduled for firmware")
				return
			}
		}
	}

	// Create test result record
	testResult := &FirmwareTestResult{
		FirmwareID:    release.ID,
		TestDeviceUID: testDeviceUID,
		TestStatus:    FirmwareTestPending,
	}

	if err := s.store.CreateFirmwareTestResult(ctx, testResult); err != nil {
		s.logger.WithError(err).Error("Failed to create test result record")
		return
	}

	// Get test device
	device, err := s.store.GetDeviceByUID(ctx, testDeviceUID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get test device")
		return
	}

	// Initiate update on test device
	if s.updateMgmt != nil {
		session, err := s.updateMgmt.InitiateUpdate(ctx, device.ID, release.ID)
		if err != nil {
			s.logger.WithError(err).Error("Failed to initiate test update")
			testResult.TestStatus = FirmwareTestFailed
			testResult.ErrorMessage = err.Error()
			s.store.UpdateFirmwareTestResult(ctx, testResult)
			return
		}

		// Update test result with session ID
		testResult.UpdateSessionID = &session.ID
		now := time.Now()
		testResult.StartedAt = &now
		testResult.TestStatus = FirmwareTestRunning
		s.store.UpdateFirmwareTestResult(ctx, testResult)

		s.logger.WithFields(logrus.Fields{
			"firmware_id": release.ID,
			"test_device": testDeviceUID,
			"session_id":  session.SessionID,
		}).Info("Firmware test initiated")
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

	// Check if firmware has passed testing before allowing approval
	if newStatus == ReleaseStatusApproved && release.ReleaseStatus != ReleaseStatusTesting {
		// Check test results
		testResults, err := s.store.GetFirmwareTestResults(ctx, id)
		if err != nil || len(testResults) == 0 {
			return ErrFirmwareNotTested
		}

		// Ensure at least one test passed
		hasPassedTest := false
		for _, test := range testResults {
			if test.TestStatus == FirmwareTestPassed {
				hasPassedTest = true
				break
			}
		}

		if !hasPassedTest {
			return ErrFirmwareNotTested
		}
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

	if !device.UpdatesEnabled && !device.IsTestDevice {
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

	// For test devices, allow any status; for production devices, require approved
	if !device.IsTestDevice && firmware.ReleaseStatus != ReleaseStatusApproved {
		return nil, ErrFirmwareNotActive
	}

	// Create update session
	session := &UpdateSession{
		SessionID:    uuid.New().String(),
		DeviceID:     deviceID,
		FirmwareID:   firmwareID,
		UpdateStatus: UpdateStatusInitiated,
		IsTestUpdate: device.IsTestDevice,
	}

	if err := s.store.CreateUpdateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create update session: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"session_id":  session.SessionID,
		"device_id":   deviceID,
		"firmware_id": firmwareID,
		"is_test":     device.IsTestDevice,
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

	if !device.Active || (!device.UpdatesEnabled && !device.IsTestDevice) {
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

		// Update test result if this was a test
		if session.IsTestUpdate {
			s.updateTestResult(ctx, session, false, "Checksum verification failed")
		}

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

	// Update test result if this was a test
	if session.IsTestUpdate {
		s.updateTestResult(ctx, session, true, "")
	}

	s.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"device_id":  session.DeviceID,
		"version":    session.Firmware.Version,
	}).Info("Update completed successfully")

	return nil
}

func (s *UpdateManagementService) updateTestResult(ctx context.Context, session *UpdateSession, success bool, errorMsg string) {
	// Find test result for this session
	testResults, err := s.store.GetFirmwareTestResults(ctx, session.FirmwareID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get test results")
		return
	}

	for _, test := range testResults {
		if test.UpdateSessionID != nil && *test.UpdateSessionID == session.ID {
			now := time.Now()
			test.CompletedAt = &now

			if success {
				test.TestStatus = FirmwareTestPassed
			} else {
				test.TestStatus = FirmwareTestFailed
				test.ErrorMessage = errorMsg
			}

			if test.StartedAt != nil {
				test.TestDuration = int64(now.Sub(*test.StartedAt).Seconds())
			}

			// Update firmware status to testing if test passed
			if success {
				firmware, err := s.store.GetFirmwareRelease(ctx, session.FirmwareID)
				if err == nil && firmware.ReleaseStatus == ReleaseStatusDraft {
					firmware.ReleaseStatus = ReleaseStatusTesting
					s.store.UpdateFirmwareRelease(ctx, firmware)
				}
			}

			if err := s.store.UpdateFirmwareTestResult(ctx, test); err != nil {
				s.logger.WithError(err).Error("Failed to update test result")
			} else {
				s.logger.WithFields(logrus.Fields{
					"firmware_id": session.FirmwareID,
					"test_status": test.TestStatus,
				}).Info("Firmware test result updated")
			}
			break
		}
	}
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
