package service

import (
	"context"
	"errors"

	"github.com/go-redis/redis/v8"

	"example.com/backstage/services/truck/internal/cache"
	"example.com/backstage/services/truck/internal/model"
	"example.com/backstage/services/truck/internal/repository"
)

// CreateOperationGroupRequest defines the request to create an operation group
type CreateOperationGroupRequest struct {
	UUID           string `json:"id" validate:"required"`
	TruckDeviceMCU string `json:"truck_uid" validate:"required"`
	Status         uint   `json:"status" validate:"required"`
	Type           uint   `json:"type" validate:"required"`
}

// OperationGroupService defines the interface for operation group service
type OperationGroupService interface {
	GetByID(ctx context.Context, id string) (*model.OperationGroup, error)
	Create(ctx context.Context, req *CreateOperationGroupRequest) (*model.OperationGroup, error)
	Update(ctx context.Context, group *model.OperationGroup) (*model.OperationGroup, error)
	FindActiveByTransportMCU(ctx context.Context, mcu string) (*model.OperationGroup, error)
	FindActiveByDeviceMCU(ctx context.Context, mcu string) (*model.OperationGroup, error)
}

// operationGroupService implements OperationGroupService
type operationGroupService struct {
	repo        repository.OperationGroupRepository
	deviceRepo  repository.DeviceRepository
	cache       cache.CacheClient
}

// NewOperationGroupService creates a new operation group service
func NewOperationGroupService(
	repo repository.OperationGroupRepository,
	deviceRepo repository.DeviceRepository,
	cache cache.CacheClient,
) OperationGroupService {
	return &operationGroupService{
		repo:        repo,
		deviceRepo:  deviceRepo,
		cache:       cache,
	}
}

// GetByID gets an operation group by ID
func (s *operationGroupService) GetByID(ctx context.Context, id string) (*model.OperationGroup, error) {
	// Try to get from cache first
	group, err := s.cache.GetOperationGroup(ctx, id)
	if err == nil {
		return group, nil
	}
	if err != redis.Nil {
		// Log the error but continue to get from database
		// logger.Warn("Failed to get operation group from cache", "error", err)
	}

	// Get from database
	group, err = s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if err := s.cache.SetOperationGroup(ctx, group); err != nil {
		// Log the error but continue
		// logger.Warn("Failed to cache operation group", "error", err)
	}

	return group, nil
}

// Create creates a new operation group
func (s *operationGroupService) Create(ctx context.Context, req *CreateOperationGroupRequest) (*model.OperationGroup, error) {
	// Find the transport device
	transportDevice, err := s.deviceRepo.FindByMCU(ctx, req.TruckDeviceMCU)
	if err != nil {
		return nil, err
	}

	// Check if the operation group already exists
	existing, err := s.repo.GetByID(ctx, req.UUID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("operation group with ID already exists")
	}

	// Create the operation group
	group := &model.OperationGroup{
		Base: model.Base{
			UUID: req.UUID,
		},
		TransportDeviceID: transportDevice.UUID,
		Type:              model.OperationType(req.Type),
		Status:            model.OperationStatus(req.Status),
	}

	// Save to database
	group, err = s.repo.Create(ctx, group)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if err := s.cache.SetOperationGroup(ctx, group); err != nil {
		// Log the error but continue
		// logger.Warn("Failed to cache operation group", "error", err)
	}

	return group, nil
}

// Update updates an operation group
func (s *operationGroupService) Update(ctx context.Context, group *model.OperationGroup) (*model.OperationGroup, error) {
	// Update in database
	group, err := s.repo.Update(ctx, group)
	if err != nil {
		return nil, err
	}

	// Update in cache
	if err := s.cache.SetOperationGroup(ctx, group); err != nil {
		// Log the error but continue
		// logger.Warn("Failed to update operation group in cache", "error", err)
	}

	// If this is an active operation group for a transport, update the cache for that too
	// We know the transport device ID from the group itself
	if group.TransportDeviceID != "" {
		// Look up the transport device to get the MCU
		device, err := s.deviceRepo.FindByUID(ctx, group.TransportDeviceID)
		if err == nil {
			// Update the active operation group cache for this transport
			if err := s.cache.SetActiveOperationGroupByTransportMCU(ctx, device.MCU, group); err != nil {
				// Log the error but continue
				// logger.Warn("Failed to update active operation group in cache", "error", err)
			}
		}
	}

	return group, nil
}

// FindActiveByTransportMCU finds an active operation group by transport MCU
func (s *operationGroupService) FindActiveByTransportMCU(ctx context.Context, mcu string) (*model.OperationGroup, error) {
	// Try to get from cache first
	group, err := s.cache.GetActiveOperationGroupByTransportMCU(ctx, mcu)
	if err == nil {
		return group, nil
	}
	if err != redis.Nil {
		// Log the error but continue to get from database
		// logger.Warn("Failed to get active operation group from cache", "error", err)
	}

	// Find the transport device
	device, err := s.deviceRepo.FindByMCU(ctx, mcu)
	if err != nil {
		return nil, err
	}

	// Find active operation group
	filter := model.OperationGroup{
		TransportDeviceID: device.UUID,
	}
	group, err = s.repo.FindActiveBy(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if err := s.cache.SetActiveOperationGroupByTransportMCU(ctx, mcu, group); err != nil {
		// Log the error but continue
		// logger.Warn("Failed to cache active operation group", "error", err)
	}

	return group, nil
}

// FindActiveByDeviceMCU finds an active operation group by device MCU
// This is more complex as operations are associated with devices, not groups directly
func (s *operationGroupService) FindActiveByDeviceMCU(ctx context.Context, mcu string) (*model.OperationGroup, error) {
	// This is not implemented in the original code either
	// It would require looking up active operations by device MCU,
	// then finding the associated operation group
	return nil, errors.New("finding operation groups by device MCU is not implemented")
}