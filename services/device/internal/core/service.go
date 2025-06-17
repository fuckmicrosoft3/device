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

// DeviceManagementService handles device lifecycle operations
type DeviceManagementService interface {
	RegisterDevice(ctx context.Context, device *Device) error
	GetDevice(ctx context.Context, id uint) (*Device, error)
	GetDeviceByUID(ctx context.Context, uid string) (*Device, error)
	ListOrganizationDevices(ctx context.Context, orgID uint) ([]*Device, error)
	UpdateDeviceStatus(ctx context.Context, id uint, active bool) error
	RecordHeartbeat(ctx context.Context, deviceUID string) error
}

// TelemetryService handles device telemetry ingestion
type TelemetryService interface {
	IngestTelemetry(ctx context.Context, deviceUID string, telemetry *Telemetry) error
	IngestBatch(ctx context.Context, batch []*Telemetry) error
	GetDeviceTelemetry(ctx context.Context, deviceID uint, limit int) ([]*Telemetry, error)
	GetIngestionStats() map[string]interface{}
}

// FirmwareManagementService handles firmware lifecycle
type FirmwareManagementService interface {
	CreateRelease(ctx context.Context, data []byte, metadata FirmwareMetadata) (*FirmwareRelease, error)
	GetRelease(ctx context.Context, id uint) (*FirmwareRelease, error)
	ListReleases(ctx context.Context, channel string) ([]*FirmwareRelease, error)
	PromoteRelease(ctx context.Context, id uint, newStatus string) error
	GetLatestRelease(ctx context.Context, channel string) (*FirmwareRelease, error)
}

// UpdateManagementService handles OTA updates
type UpdateManagementService interface {
	InitiateUpdate(ctx context.Context, deviceID, firmwareID uint) (*UpdateSession, error)
	CheckForUpdates(ctx context.Context, deviceUID, currentVersion string) (*UpdateSession, error)
	GetUpdateSession(ctx context.Context, sessionID string) (*UpdateSession, error)
	DownloadFirmwareChunk(ctx context.Context, sessionID string, offset, size int64) ([]byte, error)
	CompleteUpdate(ctx context.Context, sessionID string, checksum string) error
	GetUpdateMetrics() map[string]interface{}
}

// OrganizationService handles device tenant management
type OrganizationService interface {
	CreateOrganization(ctx context.Context, org *Organization) error
	GetOrganization(ctx context.Context, id uint) (*Organization, error)
	ListOrganizations(ctx context.Context) ([]*Organization, error)
	UpdateOrganization(ctx context.Context, org *Organization) error
}

// AuthenticationService handles API access control
type AuthenticationService interface {
	ValidateToken(ctx context.Context, token string) (*AccessToken, error)
	CreateToken(ctx context.Context, description string, scopes []string, expiresIn time.Duration) (*AccessToken, error)
	RevokeToken(ctx context.Context, token string) error
	HasScope(token *AccessToken, scope string) bool
}

// FirmwareMetadata contains metadata for firmware releases
type FirmwareMetadata struct {
	Filename       string
	Version        string
	ReleaseChannel string
	ReleaseNotes   string
}

// ServiceRegistry holds all domain services
type ServiceRegistry struct {
	DeviceManagement   DeviceManagementService
	Telemetry          TelemetryService
	FirmwareManagement FirmwareManagementService
	UpdateManagement   UpdateManagementService
	Organization       OrganizationService
	Authentication     AuthenticationService
}

// NewServiceRegistry creates all services with their dependencies
func NewServiceRegistry(cfg ServiceConfig) (*ServiceRegistry, error) {
	// Device management service
	deviceSvc := &deviceManagementService{
		store:  cfg.DataStore,
		cache:  cfg.Cache,
		logger: cfg.Logger,
	}

	// Telemetry service with async processing
	telemetrySvc := &telemetryService{
		store:     cfg.DataStore,
		messaging: cfg.Messaging,
		logger:    cfg.Logger,
		processor: NewTelemetryProcessor(cfg.DataStore, cfg.Messaging, cfg.Logger),
	}
	telemetrySvc.processor.Start(10) // 10 workers

	// Firmware management service
	firmwareSvc := &firmwareManagementService{
		store:       cfg.DataStore,
		logger:      cfg.Logger,
		storagePath: cfg.FirmwareConfig.StoragePath,
		maxFileSize: cfg.FirmwareConfig.MaxFileSize,
	}

	if cfg.FirmwareConfig.SigningEnabled && cfg.FirmwareConfig.PrivateKeyPath != "" {
		key, err := utils.LoadSigningKey(cfg.FirmwareConfig.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load signing key: %w", err)
		}
		firmwareSvc.signingKey = key
	}

	// Ensure firmware storage exists
	if err := os.MkdirAll(cfg.FirmwareConfig.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create firmware storage: %w", err)
	}

	// Update management service
	updateSvc := &updateManagementService{
		store:       cfg.DataStore,
		firmwareSvc: firmwareSvc,
		logger:      cfg.Logger,
		chunkSize:   cfg.OTAConfig.ChunkSize,
		chunkCache:  make(map[string]*cachedChunk),
	}
	go updateSvc.cleanupCache()

	// Organization service
	orgSvc := &organizationService{
		store:  cfg.DataStore,
		logger: cfg.Logger,
	}

	// Authentication service
	authSvc := &authenticationService{
		store:  cfg.DataStore,
		logger: cfg.Logger,
	}

	return &ServiceRegistry{
		DeviceManagement:   deviceSvc,
		Telemetry:          telemetrySvc,
		FirmwareManagement: firmwareSvc,
		UpdateManagement:   updateSvc,
		Organization:       orgSvc,
		Authentication:     authSvc,
	}, nil
}

// ServiceConfig holds dependencies for services
type ServiceConfig struct {
	DataStore      DataStore
	Cache          *infrastructure.Cache
	Messaging      *infrastructure.Messaging
	Logger         *logrus.Logger
	FirmwareConfig config.FirmwareConfig
	OTAConfig      config.OTAConfig
}

// --- Device Management Service Implementation ---

type deviceManagementService struct {
	store  DataStore
	cache  *infrastructure.Cache
	logger *logrus.Logger
}

func (s *deviceManagementService) RegisterDevice(ctx context.Context, device *Device) error {
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

func (s *deviceManagementService) GetDevice(ctx context.Context, id uint) (*Device, error) {
	device, err := s.store.GetDevice(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}
	return device, nil
}

func (s *deviceManagementService) GetDeviceByUID(ctx context.Context, uid string) (*Device, error) {
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

func (s *deviceManagementService) ListOrganizationDevices(ctx context.Context, orgID uint) ([]*Device, error) {
	return s.store.ListDevicesByOrganization(ctx, orgID)
}

func (s *deviceManagementService) UpdateDeviceStatus(ctx context.Context, id uint, active bool) error {
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

func (s *deviceManagementService) RecordHeartbeat(ctx context.Context, deviceUID string) error {
	device, err := s.store.GetDeviceByUID(ctx, deviceUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDeviceNotFound
		}
		return err
	}

	return s.store.UpdateDeviceHeartbeat(ctx, device.ID)
}

func (s *deviceManagementService) cacheDevice(ctx context.Context, device *Device) {
	if s.cache != nil {
		data, _ := json.Marshal(device)
		s.cache.Set(ctx, fmt.Sprintf("device:%s", device.DeviceUID), string(data), 24*time.Hour)
	}
}

func (s *deviceManagementService) getCachedDevice(ctx context.Context, uid string) (*Device, error) {
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

type telemetryService struct {
	store     DataStore
	messaging *infrastructure.Messaging
	logger    *logrus.Logger
	processor *TelemetryProcessor
}

func (s *telemetryService) IngestTelemetry(ctx context.Context, deviceUID string, telemetry *Telemetry) error {
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

	telemetry.DeviceID = device.ID
	telemetry.ReceivedAt = time.Now()

	// Enqueue for async processing
	return s.processor.Enqueue(telemetry)
}

func (s *telemetryService) IngestBatch(ctx context.Context, batch []*Telemetry) error {
	for _, telemetry := range batch {
		if telemetry.MessageID == "" {
			telemetry.MessageID = uuid.New().String()
		}
		telemetry.ReceivedAt = time.Now()

		if err := s.processor.Enqueue(telemetry); err != nil {
			s.logger.WithError(err).Warn("Failed to enqueue telemetry")
		}
	}
	return nil
}

func (s *telemetryService) GetDeviceTelemetry(ctx context.Context, deviceID uint, limit int) ([]*Telemetry, error) {
	return s.store.GetDeviceTelemetry(ctx, deviceID, limit)
}

func (s *telemetryService) GetIngestionStats() map[string]interface{} {
	return s.processor.Stats()
}

// TelemetryProcessor handles async telemetry processing
type TelemetryProcessor struct {
	store     DataStore
	messaging *infrastructure.Messaging
	logger    *logrus.Logger
	queue     chan *Telemetry
	workers   int
	wg        sync.WaitGroup
	shutdown  chan struct{}
}

func NewTelemetryProcessor(store DataStore, messaging *infrastructure.Messaging, logger *logrus.Logger) *TelemetryProcessor {
	return &TelemetryProcessor{
		store:     store,
		messaging: messaging,
		logger:    logger,
		queue:     make(chan *Telemetry, 10000),
		shutdown:  make(chan struct{}),
	}
}

func (p *TelemetryProcessor) Start(workers int) {
	p.workers = workers
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

func (p *TelemetryProcessor) Enqueue(telemetry *Telemetry) error {
	select {
	case p.queue <- telemetry:
		return nil
	default:
		return BusinessError{"TELEMETRY_001", "telemetry queue full"}
	}
}

func (p *TelemetryProcessor) Stats() map[string]interface{} {
	return map[string]interface{}{
		"queue_depth":    len(p.queue),
		"queue_capacity": cap(p.queue),
		"workers":        p.workers,
	}
}

func (p *TelemetryProcessor) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.shutdown:
			return
		case telemetry := <-p.queue:
			p.processTelemetry(telemetry)
		}
	}
}

func (p *TelemetryProcessor) processTelemetry(telemetry *Telemetry) {
	ctx := context.Background()

	err := p.store.WithTransaction(ctx, func(ctx context.Context, tx DataStore) error {
		// Save telemetry
		if err := tx.SaveTelemetry(ctx, telemetry); err != nil {
			return err
		}

		// Update device heartbeat
		if err := tx.UpdateDeviceHeartbeat(ctx, telemetry.DeviceID); err != nil {
			return err
		}

		// Publish to messaging system
		if p.messaging != nil {
			if err := p.messaging.Publish(ctx, "device-telemetry", telemetry); err != nil {
				return err
			}

			if err := tx.MarkTelemetryProcessed(ctx, telemetry.MessageID); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		p.logger.WithError(err).WithField("message_id", telemetry.MessageID).
			Error("Failed to process telemetry")
	}
}

// --- Firmware Management Service Implementation ---

type firmwareManagementService struct {
	store       DataStore
	logger      *logrus.Logger
	storagePath string
	maxFileSize int64
	signingKey  *utils.SigningKey
}

func (s *firmwareManagementService) CreateRelease(ctx context.Context, data []byte, metadata FirmwareMetadata) (*FirmwareRelease, error) {
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

func (s *firmwareManagementService) validateRelease(ctx context.Context, release *FirmwareRelease) {
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

func (s *firmwareManagementService) GetRelease(ctx context.Context, id uint) (*FirmwareRelease, error) {
	release, err := s.store.GetFirmwareRelease(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFirmwareNotFound
		}
		return nil, err
	}
	return release, nil
}

func (s *firmwareManagementService) ListReleases(ctx context.Context, channel string) ([]*FirmwareRelease, error) {
	return s.store.ListFirmwareReleases(ctx, channel)
}

func (s *firmwareManagementService) PromoteRelease(ctx context.Context, id uint, newStatus string) error {
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

func (s *firmwareManagementService) GetLatestRelease(ctx context.Context, channel string) (*FirmwareRelease, error) {
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

type updateManagementService struct {
	store       DataStore
	firmwareSvc FirmwareManagementService
	logger      *logrus.Logger
	chunkSize   int
	chunkCache  map[string]*cachedChunk
	cacheMu     sync.RWMutex
}

type cachedChunk struct {
	data       []byte
	lastAccess time.Time
}

func (s *updateManagementService) InitiateUpdate(ctx context.Context, deviceID, firmwareID uint) (*UpdateSession, error) {
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

func (s *updateManagementService) CheckForUpdates(ctx context.Context, deviceUID, currentVersion string) (*UpdateSession, error) {
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

func (s *updateManagementService) GetUpdateSession(ctx context.Context, sessionID string) (*UpdateSession, error) {
	session, err := s.store.GetUpdateSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, BusinessError{"OTA_005", "update session not found"}
		}
		return nil, err
	}
	return session, nil
}

func (s *updateManagementService) DownloadFirmwareChunk(ctx context.Context, sessionID string, offset, size int64) ([]byte, error) {
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

func (s *updateManagementService) CompleteUpdate(ctx context.Context, sessionID string, checksum string) error {
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

func (s *updateManagementService) GetUpdateMetrics() map[string]interface{} {
	// This would aggregate metrics from the database
	return map[string]interface{}{
		"cache_size": len(s.chunkCache),
		"timestamp":  time.Now(),
	}
}

func (s *updateManagementService) getCachedChunk(key string) []byte {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	if chunk, ok := s.chunkCache[key]; ok {
		chunk.lastAccess = time.Now()
		return chunk.data
	}
	return nil
}

func (s *updateManagementService) cacheChunk(key string, data []byte) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.chunkCache[key] = &cachedChunk{
		data:       data,
		lastAccess: time.Now(),
	}
}

func (s *updateManagementService) cleanupCache() {
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

type organizationService struct {
	store  DataStore
	logger *logrus.Logger
}

func (s *organizationService) CreateOrganization(ctx context.Context, org *Organization) error {
	if org.Name == "" {
		return BusinessError{"ORG_004", "organization name is required"}
	}

	if err := s.store.CreateOrganization(ctx, org); err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}

	s.logger.WithField("org_id", org.ID).Info("Organization created")
	return nil
}

func (s *organizationService) GetOrganization(ctx context.Context, id uint) (*Organization, error) {
	org, err := s.store.GetOrganization(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}
	return org, nil
}

func (s *organizationService) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	return s.store.ListOrganizations(ctx)
}

func (s *organizationService) UpdateOrganization(ctx context.Context, org *Organization) error {
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

type authenticationService struct {
	store  DataStore
	logger *logrus.Logger
}

func (s *authenticationService) ValidateToken(ctx context.Context, token string) (*AccessToken, error) {
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

func (s *authenticationService) CreateToken(ctx context.Context, description string, scopes []string, expiresIn time.Duration) (*AccessToken, error) {
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

func (s *authenticationService) RevokeToken(ctx context.Context, token string) error {
	return s.store.DeleteAccessToken(ctx, token)
}

func (s *authenticationService) HasScope(token *AccessToken, scope string) bool {
	for _, s := range token.Scopes {
		if s == scope || s == "admin" {
			return true
		}
	}
	return false
}
