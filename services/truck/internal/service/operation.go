package service

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"

	"go.novek.io/truck/internal/cache"
	"go.novek.io/truck/internal/messagebus"
	"go.novek.io/truck/internal/metrics"
	"go.novek.io/truck/internal/model"
	"go.novek.io/truck/internal/repository"
)

// CreateOperationRequest defines the request to create an operation
type CreateOperationRequest struct {
	UUID              string                 `json:"uuid" validate:"required"`
	DeviceID          string                 `json:"device_uid" validate:"required"`
	TransportDeviceID string                 `json:"transport_device_id" validate:"required"`
	Status            uint                   `json:"status" validate:"required"`
	State             string                 `json:"state" validate:"required"`
	Type              uint                   `json:"type" validate:"required"`
	Attributes        map[string]interface{} `json:"attributes"`
}

// CancelOperationRequest defines the request to cancel an operation
type CancelOperationRequest struct {
	UUID string `json:"id" validate:"required"`
}

// OperationService defines the interface for operation service
type OperationService interface {
	GetByID(ctx context.Context, id string) (*model.Operation, error)
	Create(ctx context.Context, req *CreateOperationRequest) (*model.Operation, error)
	Update(ctx context.Context, operation *model.Operation) (*model.Operation, error)
	Cancel(ctx context.Context, req *CancelOperationRequest) (*model.Operation, error)
	FindActiveByDeviceMCU(ctx context.Context, mcu string) (*model.Operation, error)
	FindActiveByTransportMCU(ctx context.Context, mcu string) (*model.Operation, error)
	FindActiveOperationsByOperationGroup(ctx context.Context, groupID string) ([]*model.Operation, error)
	GetOperationSessionByID(ctx context.Context, id string) (*model.OperationSession, error)
	CreateUpdateOperationSession(ctx context.Context, session *model.OperationSession) (*model.OperationSession, error)
	PublishOperationSessionToERP(ctx context.Context, session *model.OperationSession) error
	RepublishEvents(ctx context.Context, start, end time.Time, filter model.OperationSession) (int, error)
}

// operationService implements OperationService
type operationService struct {
	repo       repository.OperationRepository
	deviceRepo repository.DeviceRepository
	messageBus messagebus.Client
	cache      cache.CacheClient
	erpQueue   string
}

// NewOperationService creates a new operation service
func NewOperationService(
	repo repository.OperationRepository,
	deviceRepo repository.DeviceRepository,
	messageBus messagebus.Client,
	cache cache.CacheClient,
	erpQueue string,
) OperationService {
	return &operationService{
		repo:       repo,
		deviceRepo: deviceRepo,
		messageBus: messageBus,
		cache:      cache,
		erpQueue:   erpQueue,
	}
}

// GetByID gets an operation by ID
func (s *operationService) GetByID(ctx context.Context, id string) (*model.Operation, error) {
	// Try to get from cache first
	operation, err := s.cache.GetOperation(ctx, id)
	if err == nil {
		return operation, nil
	}
	if err != redis.Nil {
		// Log the error but continue to get from database
		logrus.WithError(err).Warn("Failed to get operation from cache")
	}

	// Get from database
	operation, err = s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if err := s.cache.SetOperation(ctx, operation); err != nil {
		// Log the error but continue
		logrus.WithError(err).Warn("Failed to cache operation")
	}

	return operation, nil
}

// Create creates a new operation
func (s *operationService) Create(ctx context.Context, req *CreateOperationRequest) (*model.Operation, error) {
	// Check if the operation already exists
	existing, err := s.repo.GetByID(ctx, req.UUID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("operation with ID already exists")
	}

	// Create the operation
	operation := &model.Operation{
		Base: model.Base{
			UUID: req.UUID,
		},
		DeviceID:          req.DeviceID,
		TransportDeviceID: req.TransportDeviceID,
		Type:              model.OperationType(req.Type),
		Status:            model.OperationStatus(req.Status),
		State:             req.State,
	}

	// Find the device and transport to get the MCUs
	device, err := s.deviceRepo.FindByUID(ctx, req.DeviceID)
	if err == nil {
		operation.DeviceMCU = device.MCU
	}

	transport, err := s.deviceRepo.FindByUID(ctx, req.TransportDeviceID)
	if err == nil {
		operation.TransportDeviceMCU = transport.MCU
	}

	// Save to database
	operation, err = s.repo.Create(ctx, operation)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if err := s.cache.SetOperation(ctx, operation); err != nil {
		// Log the error but continue
		logrus.WithError(err).Warn("Failed to cache operation")
	}

	// If we have the device MCU, cache the active operation for that device
	if operation.DeviceMCU != "" {
		if err := s.cache.SetActiveOperationByDeviceMCU(ctx, operation.DeviceMCU, operation); err != nil {
			// Log the error but continue
			logrus.WithError(err).Warn("Failed to cache active operation")
		}
	}

	return operation, nil
}

// Update updates an operation
func (s *operationService) Update(ctx context.Context, operation *model.Operation) (*model.Operation, error) {
	// Record metrics
	startTime := time.Now()
	collector := metrics.GetMetricsCollector()

	// Update in database
	operation, err := s.repo.Update(ctx, operation)
	if err != nil {
		collector.RecordOperation(metrics.OperationTypeFailed, time.Since(startTime))
		collector.RecordError(metrics.ErrorTypeDatabase)
		return nil, err
	}

	// Update in cache
	if err := s.cache.SetOperation(ctx, operation); err != nil {
		// Log the error but continue
		logrus.WithError(err).Warn("Failed to update operation in cache")
	}

	// If we have the device MCU, update the active operation cache for that device
	if operation.DeviceMCU != "" {
		if err := s.cache.SetActiveOperationByDeviceMCU(ctx, operation.DeviceMCU, operation); err != nil {
			// Log the error but continue
			logrus.WithError(err).Warn("Failed to update active operation in cache")
		}
	}

	// Record successful update metrics
	collector.RecordOperation(metrics.OperationTypeUpdate, time.Since(startTime))

	// Update active operations gauge (if status changed)
	count, _ := s.countActiveOperations(ctx)
	collector.SetActiveOperations(count)

	return operation, nil
}

// Cancel cancels an operation
func (s *operationService) Cancel(ctx context.Context, req *CancelOperationRequest) (*model.Operation, error) {
	// Get the operation
	operation, err := s.GetByID(ctx, req.UUID)
	if err != nil {
		return nil, err
	}

	// Update the status
	operation.Status = model.CancelledOperationStatus

	// Update the operation
	return s.Update(ctx, operation)
}

// FindActiveByDeviceMCU finds an active operation by device MCU
func (s *operationService) FindActiveByDeviceMCU(ctx context.Context, mcu string) (*model.Operation, error) {
	// Record metrics
	startTime := time.Now()
	collector := metrics.GetMetricsCollector()
	defer func() {
		collector.RecordOperation(metrics.OperationTypeCreate, time.Since(startTime))
	}()

	// Try to get from cache first
	operation, err := s.cache.GetActiveOperationByDeviceMCU(ctx, mcu)
	if err == nil {
		return operation, nil
	}
	if err != redis.Nil {
		// Log the error but continue to get from database
		logrus.WithError(err).Warn("Failed to get active operation from cache")
	}

	// Find the device
	device, err := s.deviceRepo.FindByMCU(ctx, mcu)
	if err != nil {
		return nil, err
	}

	// Find active operation
	filter := model.Operation{
		DeviceID: device.UUID,
	}
	operation, err = s.repo.GetActiveBy(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if err := s.cache.SetActiveOperationByDeviceMCU(ctx, mcu, operation); err != nil {
		// Log the error but continue
		logrus.WithError(err).Warn("Failed to cache active operation")
	}

	// Update active operations gauge
	count, _ := s.countActiveOperations(ctx)
	collector.SetActiveOperations(count)

	return operation, nil
}

// FindActiveByTransportMCU finds an active operation by transport MCU
func (s *operationService) FindActiveByTransportMCU(ctx context.Context, mcu string) (*model.Operation, error) {
	// Find the transport device
	transport, err := s.deviceRepo.FindByMCU(ctx, mcu)
	if err != nil {
		return nil, err
	}

	// Find active operation
	filter := model.Operation{
		TransportDeviceID: transport.UUID,
	}
	return s.repo.GetActiveBy(ctx, filter)
}

// FindActiveOperationsByOperationGroup finds active operations by operation group
func (s *operationService) FindActiveOperationsByOperationGroup(ctx context.Context, groupID string) ([]*model.Operation, error) {
	filter := model.Operation{
		OperationGroupID: groupID,
	}
	return s.repo.FindAllActiveBy(ctx, filter)
}

// GetOperationSessionByID gets an operation session by ID
func (s *operationService) GetOperationSessionByID(ctx context.Context, id string) (*model.OperationSession, error) {
	return s.repo.GetOperationSessionByID(ctx, id)
}

// CreateUpdateOperationSession creates or updates an operation session
func (s *operationService) CreateUpdateOperationSession(ctx context.Context, session *model.OperationSession) (*model.OperationSession, error) {
	// Save to database
	session, err := s.repo.CreateOperationSession(ctx, session)
	if err != nil {
		return nil, err
	}

	// Publish to ERP
	go func() {
		// Create a new context with a timeout
		pubCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Publish to ERP
		if err := s.PublishOperationSessionToERP(pubCtx, session); err != nil {
			logrus.WithError(err).Error("Failed to publish operation session to ERP")
		}
	}()

	return session, nil
}

// PublishOperationSessionToERP publishes an operation session to the ERP
func (s *operationService) PublishOperationSessionToERP(ctx context.Context, session *model.OperationSession) error {
	// Record metrics
	startTime := time.Now()
	collector := metrics.GetMetricsCollector()

	// Make sure we have all the needed information
	if session.Operation == nil {
		// Load the operation if not provided
		operation, err := s.repo.GetByID(ctx, session.OperationID)
		if err != nil {
			collector.RecordError(metrics.ErrorTypeDatabase)
			return err
		}
		session.Operation = operation
	}

	// Publish the message with retry
	err := messagebus.RetryWithBackoff(ctx, func() error {
		return s.messageBus.PublishMessage(ctx, session, s.erpQueue)
	}, 3)

	// Record metrics
	if err != nil {
		collector.RecordOperation(metrics.OperationTypeFailed, time.Since(startTime))
	} else {
		collector.RecordOperation(metrics.OperationTypeComplete, time.Since(startTime))
	}

	return err
}

// countActiveOperations counts active operations
func (s *operationService) countActiveOperations(ctx context.Context) (int, error) {

	// For simplicity, we'll just count operations that aren't completed, cancelled, or errored
	filter := model.Operation{}
	operations, err := s.repo.FindAllActiveBy(ctx, filter)
	if err != nil {
		return 0, err
	}

	return len(operations), nil
}

// RepublishEvents republishes operation sessions within a time range and matching a filter
func (s *operationService) RepublishEvents(ctx context.Context, start, end time.Time, filter model.OperationSession) (int, error) {
	// Record metrics
	startTime := time.Now()
	collector := metrics.GetMetricsCollector()

	// Find matching operation sessions
	sessions, err := s.repo.FindOperationSessionsBy(ctx, filter, &start, &end)
	if err != nil {
		collector.RecordError(metrics.ErrorTypeDatabase)
		return 0, err
	}

	// Publish each session to ERP
	successCount := 0
	for _, session := range sessions {
		if err := s.PublishOperationSessionToERP(ctx, session); err != nil {
			logrus.WithError(err).Errorf("Failed to republish operation session %s", session.UUID)
		} else {
			successCount++
		}
	}

	// Record metrics
	collector.RecordOperation(metrics.OperationTypeEventProcessing, time.Since(startTime))

	return len(sessions), nil
}
