package core

import (
	"context"
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
)

// Service defines the application's complete core business logic.
type Service interface {
	// Device operations
	RegisterDevice(ctx context.Context, device *Device) error
	GetDevice(ctx context.Context, id uint) (*Device, error)
	GetDeviceByUID(ctx context.Context, uid string) (*Device, error)
	ListDevices(ctx context.Context, orgID uint) ([]*Device, error)
	UpdateDeviceStatus(ctx context.Context, id uint, active bool) error

	// Message operations
	ProcessMessage(ctx context.Context, message *DeviceMessage) error
	ProcessMessageBatch(ctx context.Context, messages []*DeviceMessage) error
	GetDeviceMessages(ctx context.Context, deviceID uint, limit int) ([]*DeviceMessage, error)

	// Organization operations
	CreateOrganization(ctx context.Context, org *Organization) error
	GetOrganization(ctx context.Context, id uint) (*Organization, error)
	ListOrganizations(ctx context.Context) ([]*Organization, error)

	// Firmware & OTA operations (consolidated)
	UploadFirmware(ctx context.Context, data []byte, filename, version, releaseType string) (*FirmwareRelease, error)
	GetFirmware(ctx context.Context, id uint) (*FirmwareRelease, error)
	ListFirmware(ctx context.Context, releaseType string) ([]*FirmwareRelease, error)
	ActivateFirmware(ctx context.Context, id uint) error
	CreateOTASession(ctx context.Context, deviceID, firmwareID uint) (*OTASession, error)
	GetOTASession(ctx context.Context, sessionID string) (*OTASession, error)
	CheckDeviceUpdate(ctx context.Context, deviceUID, currentVersion string) (*OTASession, error)
	ProcessOTAChunk(ctx context.Context, sessionID string, offset, size int64) ([]byte, error)
	CompleteOTADownload(ctx context.Context, sessionID string, checksum string) error

	// Monitoring & Shutdown
	GetStats() map[string]interface{}
	Shutdown()
}

// service is the concrete implementation of the Service interface.
type service struct {
	repo       Repository
	cache      *infrastructure.Cache
	messaging  *infrastructure.Messaging
	logger     *logrus.Logger
	processor  *MessageProcessor
	fwConfig   config.FirmwareConfig
	otaConfig  config.OTAConfig
	signingKey *utils.SigningKey
	chunkCache map[string]*chunkCache // Caches firmware chunks in memory
	cacheMu    sync.RWMutex
}

type chunkCache struct {
	data       []byte
	lastAccess time.Time
}

// ServiceConfig holds all dependencies for creating the core service.
type ServiceConfig struct {
	Repository Repository
	Cache      *infrastructure.Cache
	Messaging  *infrastructure.Messaging
	Logger     *logrus.Logger
	FwConfig   config.FirmwareConfig
	OtaConfig  config.OTAConfig
}

// NewService creates a new core service instance.
func NewService(cfg ServiceConfig) (Service, error) {
	s := &service{
		repo:       cfg.Repository,
		cache:      cfg.Cache,
		messaging:  cfg.Messaging,
		logger:     cfg.Logger,
		fwConfig:   cfg.FwConfig,
		otaConfig:  cfg.OtaConfig,
		chunkCache: make(map[string]*chunkCache),
	}

	// Initialize and start the asynchronous message processor
	s.processor = NewMessageProcessor(s, cfg.Logger)
	s.processor.Start(10) // Start with 10 worker goroutines

	// Load firmware signing key if enabled
	if s.fwConfig.SigningEnabled && s.fwConfig.PrivateKeyPath != "" {
		key, err := utils.LoadSigningKey(s.fwConfig.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("could not load signing key: %w", err)
		}
		s.signingKey = key
		s.logger.Info("Firmware signing is enabled.")
	}

	// Ensure firmware storage directory exists
	if err := os.MkdirAll(s.fwConfig.StoragePath, 0o755); err != nil {
		return nil, fmt.Errorf("could not create firmware storage path: %w", err)
	}

	go s.cleanupChunkCache() // Start background task to clear stale chunk cache
	return s, nil
}

// --- Original Device/Org/Message Methods ---

func (s *service) RegisterDevice(ctx context.Context, device *Device) error {
	if device.UID == "" {
		device.UID = uuid.New().String()
	}
	if _, err := s.repo.GetDeviceByUID(ctx, device.UID); err == nil {
		return errors.New("device with this UID already exists")
	}
	if err := s.repo.CreateDevice(ctx, device); err != nil {
		return fmt.Errorf("failed to create device in db: %w", err)
	}
	s.cacheDevice(ctx, device)
	s.logger.WithField("device_uid", device.UID).Info("Device registered")
	return nil
}

func (s *service) GetDevice(ctx context.Context, id uint) (*Device, error) {
	return s.repo.GetDevice(ctx, id)
}

func (s *service) GetDeviceByUID(ctx context.Context, uid string) (*Device, error) {
	if cached, err := s.getCachedDevice(ctx, uid); err == nil && cached != nil {
		return cached, nil
	}
	device, err := s.repo.GetDeviceByUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	s.cacheDevice(ctx, device)
	return device, nil
}

func (s *service) ListDevices(ctx context.Context, orgID uint) ([]*Device, error) {
	return s.repo.ListDevices(ctx, orgID)
}

func (s *service) UpdateDeviceStatus(ctx context.Context, id uint, active bool) error {
	device, err := s.repo.GetDevice(ctx, id)
	if err != nil {
		return err
	}
	device.Active = active
	if err := s.repo.UpdateDevice(ctx, device); err != nil {
		return err
	}
	s.cacheDevice(ctx, device)
	return nil
}

func (s *service) ProcessMessage(ctx context.Context, msg *DeviceMessage) error {
	if msg.UUID == "" {
		msg.UUID = uuid.New().String()
	}
	return s.processor.Enqueue(msg)
}

func (s *service) ProcessMessageBatch(ctx context.Context, messages []*DeviceMessage) error { // Restored
	for _, msg := range messages {
		if err := s.ProcessMessage(ctx, msg); err != nil {
			s.logger.WithError(err).Warn("Failed to enqueue message from batch")
		}
	}
	return nil
}

func (s *service) GetDeviceMessages(ctx context.Context, deviceID uint, limit int) ([]*DeviceMessage, error) {
	return s.repo.ListDeviceMessages(ctx, deviceID, limit)
}

func (s *service) CreateOrganization(ctx context.Context, org *Organization) error {
	return s.repo.CreateOrganization(ctx, org)
}

func (s *service) GetOrganization(ctx context.Context, id uint) (*Organization, error) {
	return s.repo.GetOrganization(ctx, id)
}

func (s *service) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	return s.repo.ListOrganizations(ctx)
}

// --- Consolidated Firmware & OTA Methods ---

func (s *service) UploadFirmware(ctx context.Context, data []byte, filename, version, releaseType string) (*FirmwareRelease, error) {
	// Version validation from original manager.go
	if err := utils.ValidateVersion(version); err != nil {
		return nil, fmt.Errorf("invalid version: %w", err)
	}
	if int64(len(data)) > s.fwConfig.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds max %d", len(data), s.fwConfig.MaxFileSize)
	}
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])
	filePath := filepath.Join(s.fwConfig.StoragePath, fmt.Sprintf("%s_%s_%s", releaseType, version, filename))

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return nil, fmt.Errorf("failed to save firmware file: %w", err)
	}

	release := &FirmwareRelease{
		Version:     version,
		ReleaseType: releaseType,
		FilePath:    filePath,
		FileHash:    hashStr,
		FileSize:    int64(len(data)),
		Valid:       false, // Set to false, to be validated by background task
	}

	if s.signingKey != nil {
		sig, err := s.signingKey.Sign(data)
		if err != nil {
			return nil, fmt.Errorf("failed to sign firmware: %w", err)
		}
		release.Signature = sig
	}

	if err := s.repo.CreateFirmware(ctx, release); err != nil {
		os.Remove(filePath) // Clean up file if DB fails
		return nil, fmt.Errorf("failed to create firmware record: %w", err)
	}

	go s.validateFirmware(context.Background(), release) // Background validation restored
	return release, nil
}

func (s *service) validateFirmware(ctx context.Context, release *FirmwareRelease) { // Restored
	data, err := os.ReadFile(release.FilePath)
	if err != nil {
		s.logger.WithError(err).Errorf("Failed to read firmware for validation: %s", release.FilePath)
		return
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) == release.FileHash {
		release.Valid = true
		if err := s.repo.UpdateFirmware(ctx, release); err != nil {
			s.logger.WithError(err).Error("Failed to mark firmware as valid")
		}
	}
}

func (s *service) GetFirmware(ctx context.Context, id uint) (*FirmwareRelease, error) {
	return s.repo.GetFirmware(ctx, id)
}

func (s *service) ListFirmware(ctx context.Context, releaseType string) ([]*FirmwareRelease, error) {
	return s.repo.ListFirmware(ctx, releaseType)
}

func (s *service) ActivateFirmware(ctx context.Context, id uint) error {
	fw, err := s.repo.GetFirmware(ctx, id)
	if err != nil {
		return fmt.Errorf("firmware not found: %w", err)
	}
	if !fw.Valid {
		return errors.New("cannot activate an invalid firmware release")
	}
	fw.Active = true
	return s.repo.UpdateFirmware(ctx, fw)
}

func (s *service) CreateOTASession(ctx context.Context, deviceID, firmwareID uint) (*OTASession, error) {
	device, err := s.repo.GetDevice(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}
	if !device.Active || !device.AllowUpdates {
		return nil, errors.New("device cannot receive updates")
	}
	firmware, err := s.repo.GetFirmware(ctx, firmwareID)
	if err != nil {
		return nil, fmt.Errorf("firmware not found: %w", err)
	}
	if !firmware.Active || !firmware.Valid {
		return nil, errors.New("firmware not available for update")
	}
	session := &OTASession{
		SessionID:  uuid.New().String(),
		DeviceID:   deviceID,
		FirmwareID: firmwareID,
		Status:     "pending",
	}
	if err := s.repo.CreateOTASession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create ota session: %w", err)
	}
	return s.repo.GetOTASession(ctx, session.SessionID)
}

func (s *service) GetOTASession(ctx context.Context, sessionID string) (*OTASession, error) {
	return s.repo.GetOTASession(ctx, sessionID)
}

func (s *service) CheckDeviceUpdate(ctx context.Context, deviceUID, currentVersion string) (*OTASession, error) {
	device, err := s.repo.GetDeviceByUID(ctx, deviceUID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}
	if !device.Active || !device.AllowUpdates {
		return nil, nil
	}
	if sessions, err := s.repo.ListDeviceOTASessions(ctx, device.ID); err == nil { // Logic from original ota.go
		for _, session := range sessions {
			if session.Status == "pending" || session.Status == "downloading" {
				return session, nil
			}
		}
	}
	latest, err := s.repo.GetLatestActiveFirmware(ctx, "production")
	if err != nil {
		return nil, nil
	}
	if utils.CompareVersions(currentVersion, latest.Version) >= 0 { // Version comparison restored
		return nil, nil
	}
	return s.CreateOTASession(ctx, device.ID, latest.ID)
}

func (s *service) ProcessOTAChunk(ctx context.Context, sessionID string, offset, size int64) ([]byte, error) {
	session, err := s.repo.GetOTASession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	if session.Status == "pending" {
		session.Status = "downloading"
		now := time.Now()
		session.StartedAt = &now
		s.repo.UpdateOTASession(ctx, session)
	} else if session.Status != "downloading" {
		return nil, fmt.Errorf("session not in downloadable state, status: %s", session.Status)
	}

	cacheKey := fmt.Sprintf("%s:%d:%d", sessionID, offset, size)
	if cached := s.getFromCache(cacheKey); cached != nil {
		return cached, nil
	}
	file, err := os.Open(session.Firmware.FilePath)
	if err != nil {
		return nil, fmt.Errorf("could not open firmware file: %w", err)
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek in file: %w", err)
	}
	chunk := make([]byte, size)
	n, readErr := io.ReadFull(file, chunk)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("failed to read chunk: %w", readErr)
	}
	chunk = chunk[:n]
	session.BytesDownloaded = offset + int64(n)
	session.Progress = int(float64(session.BytesDownloaded) / float64(session.Firmware.FileSize) * 100)
	s.repo.UpdateOTASession(ctx, session)
	s.addToCache(cacheKey, chunk)
	return chunk, nil
}

func (s *service) CompleteOTADownload(ctx context.Context, sessionID, checksum string) error {
	session, err := s.repo.GetOTASession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	if session.Status != "downloading" {
		return errors.New("invalid session state for completion")
	}
	if checksum != session.Firmware.FileHash {
		session.Status = "failed"
		session.ErrorMessage = "checksum mismatch"
		s.repo.UpdateOTASession(ctx, session)
		return errors.New(session.ErrorMessage)
	}
	session.Status = "completed"
	now := time.Now()
	session.CompletedAt = &now
	session.Progress = 100
	if err := s.repo.UpdateOTASession(ctx, session); err != nil {
		return err
	}
	if device, err := s.repo.GetDevice(ctx, session.DeviceID); err == nil {
		device.FirmwareVersion = session.Firmware.Version
		s.repo.UpdateDevice(ctx, device)
	}
	return nil
}

// --- Helper, Monitoring & Shutdown ---

func (s *service) cacheDevice(ctx context.Context, device *Device) {
	data, _ := json.Marshal(device)
	s.cache.Set(ctx, "device:"+device.UID, string(data), 24*time.Hour)
}

func (s *service) getCachedDevice(ctx context.Context, uid string) (*Device, error) {
	data, err := s.cache.Get(ctx, "device:"+uid)
	if err != nil {
		return nil, err
	}
	var device Device
	return &device, json.Unmarshal([]byte(data), &device)
}

func (s *service) getFromCache(key string) []byte {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	if c, ok := s.chunkCache[key]; ok {
		c.lastAccess = time.Now()
		return c.data
	}
	return nil
}

func (s *service) addToCache(key string, data []byte) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.chunkCache[key] = &chunkCache{data: data, lastAccess: time.Now()}
}

func (s *service) cleanupChunkCache() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.cacheMu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for key, cache := range s.chunkCache {
			if cache.lastAccess.Before(cutoff) {
				delete(s.chunkCache, key)
			}
		}
		s.cacheMu.Unlock()
	}
}

func (s *service) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"processor": s.processor.Stats(),
		"timestamp": time.Now(),
	}
}

func (s *service) Shutdown() {
	s.logger.Info("Core service shutting down.")
	s.processor.Stop()
}

// --- Asynchronous Message Processor (Full implementation restored) ---

type MessageProcessor struct {
	service  *service
	logger   *logrus.Logger
	queue    chan *DeviceMessage
	workers  int
	wg       sync.WaitGroup
	shutdown chan struct{}
}

func NewMessageProcessor(svc *service, logger *logrus.Logger) *MessageProcessor {
	return &MessageProcessor{
		service:  svc,
		logger:   logger,
		queue:    make(chan *DeviceMessage, 10000),
		shutdown: make(chan struct{}),
	}
}

func (mp *MessageProcessor) Start(workers int) {
	mp.workers = workers
	for i := range workers {
		mp.wg.Add(1)
		go mp.worker(i)
	}
	mp.logger.Infof("Started %d message processor workers", workers)
}

func (mp *MessageProcessor) Stop() {
	close(mp.shutdown)
	mp.wg.Wait()
	mp.logger.Info("Message processor stopped")
}

func (mp *MessageProcessor) Enqueue(msg *DeviceMessage) error {
	select {
	case mp.queue <- msg:
		return nil
	default:
		return errors.New("message queue full")
	}
}

func (mp *MessageProcessor) Stats() map[string]interface{} {
	return map[string]interface{}{
		"queue_length":   len(mp.queue),
		"queue_capacity": cap(mp.queue),
		"workers":        mp.workers,
	}
}

func (mp *MessageProcessor) worker(id int) {
	defer mp.wg.Done()
	for {
		select {
		case <-mp.shutdown:
			return
		case msg := <-mp.queue:
			mp.process(msg)
		}
	}
}

func (mp *MessageProcessor) process(msg *DeviceMessage) {
	ctx := context.Background()
	device, err := mp.service.repo.GetDeviceByUID(ctx, msg.Device.UID)
	if err != nil {
		mp.logger.WithError(err).Errorf("Device not found for message: %s", msg.Device.UID)
		return
	}
	msg.DeviceID = device.ID
	err = mp.service.repo.WithTransaction(ctx, func(ctx context.Context, repo Repository) error {
		if err := repo.CreateMessage(ctx, msg); err != nil {
			return err
		}
		if err := repo.UpdateDeviceLastSeen(ctx, device.ID); err != nil {
			return err
		}
		if mp.service.messaging != nil {
			if err := mp.service.messaging.Publish(ctx, "device-messages", msg); err != nil {
				return err
			}
			if err := repo.MarkMessagePublished(ctx, msg.UUID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		mp.logger.WithError(err).Error("Failed to process message in transaction")
	}
}
