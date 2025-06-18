package service

import (
	"context"

	"go.novek.io/truck/internal/cache"
	"go.novek.io/truck/internal/model"
	"go.novek.io/truck/internal/repository"
)

// DeviceService defines the interface for device service
type DeviceService interface {
	FindByUID(ctx context.Context, uid string) (*model.Device, error)
	FindByMCU(ctx context.Context, mcu string) (*model.Device, error)
	FindOrCreateDeviceByMCU(ctx context.Context, mcu string) (*model.Device, error)
	FindOrCreateTransportByMCU(ctx context.Context, mcu string) (*model.Device, error)
}

// deviceService implements DeviceService
type deviceService struct {
	repo  repository.DeviceRepository
	cache cache.CacheClient
}

// NewDeviceService creates a new device service
func NewDeviceService(repo repository.DeviceRepository, cache cache.CacheClient) DeviceService {
	return &deviceService{
		repo:  repo,
		cache: cache,
	}
}

// FindByUID finds a device by UID
func (s *deviceService) FindByUID(ctx context.Context, uid string) (*model.Device, error) {
	return s.repo.FindByUID(ctx, uid)
}

// FindByMCU finds a device by MCU
func (s *deviceService) FindByMCU(ctx context.Context, mcu string) (*model.Device, error) {
	return s.repo.FindByMCU(ctx, mcu)
}

// FindOrCreateDeviceByMCU finds or creates a device by MCU
func (s *deviceService) FindOrCreateDeviceByMCU(ctx context.Context, mcu string) (*model.Device, error) {
	return s.repo.FindOrCreateDeviceByMCU(ctx, mcu)
}

// FindOrCreateTransportByMCU finds or creates a transport by MCU
func (s *deviceService) FindOrCreateTransportByMCU(ctx context.Context, mcu string) (*model.Device, error) {
	return s.repo.FindOrCreateTransportByMCU(ctx, mcu)
}
