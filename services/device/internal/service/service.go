package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"time"
	
	"example.com/backstage/services/device/internal/cache"
	"example.com/backstage/services/device/internal/database"
	"example.com/backstage/services/device/internal/messaging"
	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/repository"
	
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Service defines the business logic operations
type Service interface {
	// Device operations
	RegisterDevice(ctx context.Context, device *models.Device) error
	GetDevice(ctx context.Context, id uint) (*models.Device, error)
	GetDeviceByUID(ctx context.Context, uid string) (*models.Device, error)
	ListDevices(ctx context.Context, orgID uint) ([]*models.Device, error)
	UpdateDeviceStatus(ctx context.Context, id uint, active bool) error
	AssignFirmwareToDevice(ctx context.Context, deviceID, releaseID uint) error
	
	// DeviceMessage operations
	ProcessDeviceMessage(ctx context.Context, message *models.DeviceMessage) error
	GetDeviceMessages(ctx context.Context, deviceID uint, limit int) ([]*models.DeviceMessage, error)
	
	// Organization operations
	CreateOrganization(ctx context.Context, org *models.Organization) error
	UpdateOrganization(ctx context.Context, org *models.Organization) error
	GetOrganization(ctx context.Context, id uint) (*models.Organization, error)
	ListOrganizations(ctx context.Context) ([]*models.Organization, error)
	
	// FirmwareRelease operations
	CreateFirmwareRelease(ctx context.Context, release *models.FirmwareRelease) error
	GetFirmwareRelease(ctx context.Context, id uint) (*models.FirmwareRelease, error)
	ListFirmwareReleases(ctx context.Context, releaseType models.ReleaseType) ([]*models.FirmwareRelease, error)
	ActivateFirmwareRelease(ctx context.Context, id uint) error
	
	// Enhanced operations for batch processing and monitoring
	BatchProcessMessages(ctx context.Context, messages []*models.DeviceMessage) error
	GetProcessorStats() map[string]interface{}
	Shutdown() error

	// OTA Update API methods
	CreateUpdateSession(ctx context.Context, deviceID, firmwareID uint, priority uint, forceUpdate, allowRollback bool) (*models.OTAUpdateSession, error)
	GetUpdateSession(ctx context.Context, sessionID string) (*models.OTAUpdateSession, error)
	ListDeviceUpdateSessions(ctx context.Context, deviceID uint, limit int) ([]*models.OTAUpdateSession, error)
	CancelUpdateSession(ctx context.Context, sessionID string) error
	CreateUpdateBatch(ctx context.Context, firmwareID uint, deviceIDs []uint, priority uint, forceUpdate, allowRollback bool, maxConcurrent uint) (*models.OTAUpdateBatch, error)
	CheckForUpdate(ctx context.Context, deviceUID string, currentVersion string) (*models.OTAUpdateSession, error)
	AcknowledgeUpdate(ctx context.Context, sessionID string) error
	GetUpdateChunk(ctx context.Context, sessionID string, offset, size uint64) ([]byte, error)
	CompleteDownload(ctx context.Context, sessionID string, checksum string) error
	CompleteUpdate(ctx context.Context, sessionID string, success bool, errorMessage string) error
	GetStuckUpdates(ctx context.Context, thresholdMinutes int) ([]*models.OTAUpdateSession, error)
	GetUpdateStats(ctx context.Context) (map[string]interface{}, error)

	// Enhanced Firmware Management API methods
	UploadFirmware(ctx context.Context, file io.Reader, filename string, releaseType models.ReleaseType, version string, isTest bool, notes string) (*models.FirmwareReleaseExtended, error)
	ValidateFirmware(ctx context.Context, releaseID uint) (*models.FirmwareReleaseValidation, error)
	SignFirmware(ctx context.Context, releaseID uint, privateKeyPEM string) error
	PromoteTestToProduction(ctx context.Context, testReleaseID uint) (*models.FirmwareReleaseExtended, error)
	ParseVersion(version string) (*models.SemanticVersion, error)
	CompareVersions(v1, v2 string) (int, error)
	GetFirmwareByVersion(ctx context.Context, version string, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error)
	GetLatestFirmware(ctx context.Context, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error)
	GetFirmwareFile(ctx context.Context, releaseID uint) (string, io.ReadCloser, error)
}

// service is an implementation of the Service interface
type service struct {
	repo            repository.Repository
	cache           cache.RedisClient
	messagingClient messaging.ServiceBusClient
	log             *logrus.Logger
	msgProcessor    *MessageProcessor
	
	// Added Services
	firmwareService FirmwareService
	otaService      OTAService
}

// ServiceConfig holds the configuration for the service
type ServiceConfig struct {
	Repository      repository.Repository
	Cache           cache.RedisClient
	MessagingClient messaging.ServiceBusClient
	Logger          *logrus.Logger
	StoragePath     string
}

// NewService creates a new service instance with improved configuration
func NewService(config ServiceConfig) (Service, error) {
	// Validate required config
	if config.Repository == nil {
		return nil, errors.New("repository is required")
	}
	if config.Cache == nil {
		return nil, errors.New("cache is required")
	}
	if config.MessagingClient == nil {
		return nil, errors.New("messaging client is required")
	}
	if config.Logger == nil {
		config.Logger = logrus.New() // Default logger
	}
	if config.StoragePath == "" {
		config.StoragePath = "/var/firmware" // Default storage path
	}

	// Calculate optimal worker count based on available CPUs
	workerCount := runtime.NumCPU() * 2
	if workerCount < 4 {
		workerCount = 4 // Minimum 4 workers
	}
	
	// Create message processor for asynchronous processing
	msgProcessor := NewMessageProcessor(
		config.Repository,
		config.Cache,
		config.MessagingClient,
		config.Logger,
		workerCount,
	)
	
	// Extract the DB connection for specialized repositories
	var db database.DB
	if repoImpl, ok := config.Repository.(*repository.repo); ok {
		db = repoImpl.db
	} else {
		return nil, errors.New("unable to get database connection from repository")
	}
	
	// Create specialized repositories
	firmwareRepo := repository.NewFirmwareRepository(db)
	otaRepo := repository.NewOTARepository(db)
	
	// Create firmware service
	firmwareService, err := NewFirmwareService(
		config.Repository,
		firmwareRepo,
		config.Logger,
		config.StoragePath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create firmware service: %w", err)
	}
	
	// Create OTA service
	otaService := NewOTAService(
		config.Repository,
		otaRepo,
		firmwareRepo,
		firmwareService,
		config.Logger,
	)
	
	return &service{
		repo:            config.Repository,
		cache:           config.Cache,
		messagingClient: config.MessagingClient,
		log:             config.Logger,
		msgProcessor:    msgProcessor,
		firmwareService: firmwareService,
		otaService:      otaService,
	}, nil
}

// Legacy constructor for backward compatibility
func NewServiceLegacy(
	repo repository.Repository, 
	cache cache.RedisClient,
	messagingClient messaging.ServiceBusClient,
	log *logrus.Logger,
) (Service, error) {
	return NewService(ServiceConfig{
		Repository:      repo,
		Cache:           cache,
		MessagingClient: messagingClient,
		Logger:          log,
		StoragePath:     "/var/firmware",
	})
}

// Device operations implementation
func (s *service) RegisterDevice(ctx context.Context, device *models.Device) error {
	// Generate UUID if not provided
	if device.UID == "" {
		device.UID = uuid.New().String()
	}
	
	// Set default values
	device.Active = true
	device.AllowUpdates = true
	
	if err := s.repo.CreateDevice(ctx, device); err != nil {
		return fmt.Errorf("failed to create device: %w", err)
	}
	
	// Cache the device info
	deviceJSON, err := json.Marshal(device)
	if err == nil {
		s.cache.Set(ctx, fmt.Sprintf("device:%s", device.UID), string(deviceJSON), 24*time.Hour)
	}
	
	return nil
}

func (s *service) GetDevice(ctx context.Context, id uint) (*models.Device, error) {
	// Try to get from database
	device, err := s.repo.FindDeviceByID(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Update cache
	deviceJSON, err := json.Marshal(device)
	if err == nil {
		s.cache.Set(ctx, fmt.Sprintf("device:%s", device.UID), string(deviceJSON), 24*time.Hour)
	}
	
	return device, nil
}

func (s *service) GetDeviceByUID(ctx context.Context, uid string) (*models.Device, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("device:%s", uid)
	cachedData, err := s.cache.Get(ctx, cacheKey)
	if err == nil {
		var device models.Device
		if err := json.Unmarshal([]byte(cachedData), &device); err == nil {
			return &device, nil
		}
	}
	
	// Fallback to database
	device, err := s.repo.FindDeviceByUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	
	// Update cache
	deviceJSON, err := json.Marshal(device)
	if err == nil {
		s.cache.Set(ctx, cacheKey, string(deviceJSON), 24*time.Hour)
	}
	
	return device, nil
}

func (s *service) ListDevices(ctx context.Context, orgID uint) ([]*models.Device, error) {
	return s.repo.ListDevices(ctx, orgID)
}

func (s *service) UpdateDeviceStatus(ctx context.Context, id uint, active bool) error {
	device, err := s.repo.FindDeviceByID(ctx, id)
	if err != nil {
		return err
	}
	
	device.Active = active
	
	if err := s.repo.UpdateDevice(ctx, device); err != nil {
		return err
	}
	
	// Update cache
	deviceJSON, err := json.Marshal(device)
	if err == nil {
		s.cache.Set(ctx, fmt.Sprintf("device:%s", device.UID), string(deviceJSON), 24*time.Hour)
	}
	
	return nil
}

func (s *service) AssignFirmwareToDevice(ctx context.Context, deviceID, releaseID uint) error {
	device, err := s.repo.FindDeviceByID(ctx, deviceID)
	if err != nil {
		return err
	}
	
	release, err := s.repo.FindFirmwareReleaseByID(ctx, releaseID)
	if err != nil {
		return err
	}
	
	// Check if release is active
	if !release.Active {
		return fmt.Errorf("cannot assign inactive firmware release")
	}
	
	// Assign the release to the device
	device.CurrentReleaseID = &release.ID
	
	if err := s.repo.UpdateDevice(ctx, device); err != nil {
		return err
	}
	
	// Update cache
	deviceJSON, err := json.Marshal(device)
	if err == nil {
		s.cache.Set(ctx, fmt.Sprintf("device:%s", device.UID), string(deviceJSON), 24*time.Hour)
	}
	
	return nil
}

// DeviceMessage operations implementation

func (s *service) ProcessDeviceMessage(ctx context.Context, message *models.DeviceMessage) error {
	// Ensure the message has a UUID
	if message.UUID == "" {
		message.UUID = uuid.New().String()
	}
	
	// Use the message processor for asynchronous processing
	return s.msgProcessor.EnqueueMessage(message)
}

// BatchProcessMessages processes multiple messages in a batch
func (s *service) BatchProcessMessages(ctx context.Context, messages []*models.DeviceMessage) error {
	if len(messages) == 0 {
		return nil
	}
	
	s.log.Infof("Processing batch of %d messages", len(messages))
	
	// Enqueue each message for processing
	for i, msg := range messages {
		if msg.UUID == "" {
			msg.UUID = uuid.New().String()
		}
		
		if err := s.msgProcessor.EnqueueMessage(msg); err != nil {
			s.log.WithError(err).Errorf("Failed to enqueue message %d/%d", i+1, len(messages))
			return err
		}
	}
	
	return nil
}

// GetProcessorStats returns statistics about the message processor
func (s *service) GetProcessorStats() map[string]interface{} {
	return s.msgProcessor.QueueStats()
}

// Shutdown gracefully stops the service
func (s *service) Shutdown() error {
	s.log.Info("Shutting down service...")
	s.msgProcessor.Stop()
	return nil
}

func (s *service) GetDeviceMessages(ctx context.Context, deviceID uint, limit int) ([]*models.DeviceMessage, error) {
	return s.repo.ListDeviceMessages(ctx, deviceID, limit)
}

// Organization operations implementation

func (s *service) CreateOrganization(ctx context.Context, org *models.Organization) error {
	// Set default values
	org.Active = true
	
	return s.repo.CreateOrganization(ctx, org)
}

func (s *service) UpdateOrganization(ctx context.Context, org *models.Organization) error {
	return s.repo.UpdateOrganization(ctx, org)
}

func (s *service) GetOrganization(ctx context.Context, id uint) (*models.Organization, error) {
	return s.repo.FindOrganizationByID(ctx, id)
}

func (s *service) ListOrganizations(ctx context.Context) ([]*models.Organization, error) {
	return s.repo.ListOrganizations(ctx)
}

// FirmwareRelease operations implementation

func (s *service) CreateFirmwareRelease(ctx context.Context, release *models.FirmwareRelease) error {
	// Set default values if not provided
	if release.ReleaseType == "" {
		release.ReleaseType = models.ReleaseTypeDevelopment
	}
	
	// Validate the release
	if release.FilePath == "" {
		return fmt.Errorf("file path is required")
	}
	
	if release.Version == "" {
		return fmt.Errorf("version is required")
	}
	
	// Create the release
	return s.repo.CreateFirmwareRelease(ctx, release)
}

func (s *service) GetFirmwareRelease(ctx context.Context, id uint) (*models.FirmwareRelease, error) {
	return s.repo.FindFirmwareReleaseByID(ctx, id)
}

func (s *service) ListFirmwareReleases(ctx context.Context, releaseType models.ReleaseType) ([]*models.FirmwareRelease, error) {
	return s.repo.ListFirmwareReleases(ctx, releaseType)
}

func (s *service) ActivateFirmwareRelease(ctx context.Context, id uint) error {
	release, err := s.repo.FindFirmwareReleaseByID(ctx, id)
	if err != nil {
		return err
	}
	
	// Check if the release can be activated
	if !release.Valid {
		return fmt.Errorf("cannot activate invalid release")
	}
	
	if release.IsTest && !release.TestPassed {
		return fmt.Errorf("cannot activate test release that has not passed testing")
	}
	
	// Activate the release
	release.Active = true
	
	return s.repo.UpdateFirmwareRelease(ctx, release)
}

// OTA Update Service methods - delegated to otaService
func (s *service) CreateUpdateSession(ctx context.Context, deviceID, firmwareID uint, priority uint, forceUpdate, allowRollback bool) (*models.OTAUpdateSession, error) {
	return s.otaService.CreateUpdateSession(ctx, deviceID, firmwareID, priority, forceUpdate, allowRollback)
}

func (s *service) GetUpdateSession(ctx context.Context, sessionID string) (*models.OTAUpdateSession, error) {
	return s.otaService.GetUpdateSession(ctx, sessionID)
}

func (s *service) ListDeviceUpdateSessions(ctx context.Context, deviceID uint, limit int) ([]*models.OTAUpdateSession, error) {
	return s.otaService.ListDeviceUpdateSessions(ctx, deviceID, limit)
}

func (s *service) CancelUpdateSession(ctx context.Context, sessionID string) error {
	return s.otaService.CancelUpdateSession(ctx, sessionID)
}

func (s *service) CreateUpdateBatch(ctx context.Context, firmwareID uint, deviceIDs []uint, priority uint, forceUpdate, allowRollback bool, maxConcurrent uint) (*models.OTAUpdateBatch, error) {
	return s.otaService.CreateUpdateBatch(ctx, firmwareID, deviceIDs, priority, forceUpdate, allowRollback, maxConcurrent)
}

func (s *service) CheckForUpdate(ctx context.Context, deviceUID string, currentVersion string) (*models.OTAUpdateSession, error) {
	return s.otaService.CheckForUpdate(ctx, deviceUID, currentVersion)
}

func (s *service) AcknowledgeUpdate(ctx context.Context, sessionID string) error {
	return s.otaService.AcknowledgeUpdate(ctx, sessionID)
}

func (s *service) GetUpdateChunk(ctx context.Context, sessionID string, offset, size uint64) ([]byte, error) {
	return s.otaService.GetUpdateChunk(ctx, sessionID, offset, size)
}

func (s *service) CompleteDownload(ctx context.Context, sessionID string, checksum string) error {
	return s.otaService.CompleteDownload(ctx, sessionID, checksum)
}

func (s *service) CompleteUpdate(ctx context.Context, sessionID string, success bool, errorMessage string) error {
	return s.otaService.CompleteUpdate(ctx, sessionID, success, errorMessage)
}

func (s *service) GetStuckUpdates(ctx context.Context, thresholdMinutes int) ([]*models.OTAUpdateSession, error) {
	return s.otaService.GetStuckUpdates(ctx, thresholdMinutes)
}

func (s *service) GetUpdateStats(ctx context.Context) (map[string]interface{}, error) {
	return s.otaService.GetUpdateStats(ctx)
}

// Firmware Service methods - delegated to firmwareService
func (s *service) UploadFirmware(ctx context.Context, file io.Reader, filename string, releaseType models.ReleaseType, version string, isTest bool, notes string) (*models.FirmwareReleaseExtended, error) {
	return s.firmwareService.UploadFirmware(ctx, file, filename, releaseType, version, isTest, notes)
}

func (s *service) ValidateFirmware(ctx context.Context, releaseID uint) (*models.FirmwareReleaseValidation, error) {
	return s.firmwareService.ValidateFirmware(ctx, releaseID)
}

func (s *service) SignFirmware(ctx context.Context, releaseID uint, privateKeyPEM string) error {
	return s.firmwareService.SignFirmware(ctx, releaseID, privateKeyPEM)
}

func (s *service) PromoteTestToProduction(ctx context.Context, testReleaseID uint) (*models.FirmwareReleaseExtended, error) {
	return s.firmwareService.PromoteTestToProduction(ctx, testReleaseID)
}

func (s *service) ParseVersion(version string) (*models.SemanticVersion, error) {
	return s.firmwareService.ParseVersion(version)
}

func (s *service) CompareVersions(v1, v2 string) (int, error) {
	return s.firmwareService.CompareVersions(v1, v2)
}

func (s *service) GetFirmwareByVersion(ctx context.Context, version string, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error) {
	return s.firmwareService.GetFirmwareByVersion(ctx, version, releaseType)
}

func (s *service) GetLatestFirmware(ctx context.Context, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error) {
	return s.firmwareService.GetLatestFirmware(ctx, releaseType)
}

func (s *service) GetFirmwareFile(ctx context.Context, releaseID uint) (string, io.ReadCloser, error) {
	return s.firmwareService.GetFirmwareFile(ctx, releaseID)
}
