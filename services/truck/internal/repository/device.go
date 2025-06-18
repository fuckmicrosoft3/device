package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.novek.io/truck/internal/db"
	"go.novek.io/truck/internal/model"
)

// DeviceRepository defines the interface for device repository
type DeviceRepository interface {
	FindByUID(ctx context.Context, uid string) (*model.Device, error)
	FindByMCU(ctx context.Context, mcu string) (*model.Device, error)
	FindOrCreateDeviceByMCU(ctx context.Context, mcu string) (*model.Device, error)
	FindOrCreateTransportByMCU(ctx context.Context, mcu string) (*model.Device, error)
	Create(ctx context.Context, device *model.Device) error
}

// deviceRepository implements DeviceRepository
type deviceRepository struct {
	db *gorm.DB
}

// NewDeviceRepository creates a new device repository
func NewDeviceRepository(db *gorm.DB) DeviceRepository {
	return &deviceRepository{db: db}
}

// FindByUID finds a device by UID
func (r *deviceRepository) FindByUID(ctx context.Context, uid string) (*model.Device, error) {
	var device model.Device
	err := r.db.WithContext(ctx).Where("uuid = ?", uid).First(&device).Error
	if err != nil {
		if db.IsRecordNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &device, nil
}

// FindByMCU finds a device by MCU
func (r *deviceRepository) FindByMCU(ctx context.Context, mcu string) (*model.Device, error) {
	var device model.Device
	err := r.db.WithContext(ctx).Where("LOWER(mcu) = LOWER(?)", mcu).First(&device).Error
	if err != nil {
		if db.IsRecordNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &device, nil
}

// findOrCreateByMcuAndType finds or creates a device by MCU and type
func (r *deviceRepository) findOrCreateByMcuAndType(ctx context.Context, mcu string, deviceType model.DeviceType) (*model.Device, error) {
	device, err := r.FindByMCU(ctx, mcu)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Create new device if not found
			device = &model.Device{
				Base: model.Base{
					UUID: uuid.New().String(),
				},
				MCU:  mcu,
				Type: deviceType,
			}
			if err := r.Create(ctx, device); err != nil {
				return nil, err
			}
			return device, nil
		}
		return nil, err
	}
	return device, nil
}

// FindOrCreateDeviceByMCU finds or creates a device by MCU
func (r *deviceRepository) FindOrCreateDeviceByMCU(ctx context.Context, mcu string) (*model.Device, error) {
	return r.findOrCreateByMcuAndType(ctx, mcu, model.MachineType)
}

// FindOrCreateTransportByMCU finds or creates a transport by MCU
func (r *deviceRepository) FindOrCreateTransportByMCU(ctx context.Context, mcu string) (*model.Device, error) {
	return r.findOrCreateByMcuAndType(ctx, mcu, model.TransportType)
}

// Create creates a new device
func (r *deviceRepository) Create(ctx context.Context, device *model.Device) error {
	// Normalize MCU to prevent duplicates with different casing
	device.MCU = strings.ToUpper(device.MCU)

	return r.db.WithContext(ctx).Create(device).Error
}
