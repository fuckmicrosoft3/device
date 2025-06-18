package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"go.novek.io/truck/internal/db"
	"go.novek.io/truck/internal/model"
)

// OperationGroupRepository defines the interface for operation group repository
type OperationGroupRepository interface {
	Create(ctx context.Context, group *model.OperationGroup) (*model.OperationGroup, error)
	Update(ctx context.Context, group *model.OperationGroup) (*model.OperationGroup, error)
	GetByID(ctx context.Context, id string) (*model.OperationGroup, error)
	FindBy(ctx context.Context, filter model.OperationGroup) (*model.OperationGroup, error)
	FindActiveBy(ctx context.Context, filter model.OperationGroup) (*model.OperationGroup, error)
}

// operationGroupRepository implements OperationGroupRepository
type operationGroupRepository struct {
	db *gorm.DB
}

// NewOperationGroupRepository creates a new operation group repository
func NewOperationGroupRepository(db *gorm.DB) OperationGroupRepository {
	return &operationGroupRepository{db: db}
}

// Create creates a new operation group
func (r *operationGroupRepository) Create(ctx context.Context, group *model.OperationGroup) (*model.OperationGroup, error) {
	if err := r.db.WithContext(ctx).Create(group).Error; err != nil {
		return nil, err
	}
	return group, nil
}

// Update updates an operation group
func (r *operationGroupRepository) Update(ctx context.Context, group *model.OperationGroup) (*model.OperationGroup, error) {
	if err := r.db.WithContext(ctx).Updates(group).Error; err != nil {
		return nil, err
	}
	return group, nil
}

// GetByID gets an operation group by ID
func (r *operationGroupRepository) GetByID(ctx context.Context, id string) (*model.OperationGroup, error) {
	var group model.OperationGroup
	err := r.db.WithContext(ctx).Where("uuid = ?", id).First(&group).Error
	if err != nil {
		if db.IsRecordNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &group, nil
}

// FindBy finds an operation group by filter
func (r *operationGroupRepository) FindBy(ctx context.Context, filter model.OperationGroup) (*model.OperationGroup, error) {
	var group model.OperationGroup
	err := r.db.WithContext(ctx).
		Preload("Operations").
		Where(&filter).
		First(&group).Error

	if err != nil {
		if db.IsRecordNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &group, nil
}

// FindActiveBy finds an active operation group by filter
func (r *operationGroupRepository) FindActiveBy(ctx context.Context, filter model.OperationGroup) (*model.OperationGroup, error) {
	var group model.OperationGroup
	err := r.db.WithContext(ctx).
		Where("status NOT IN (?)", []model.OperationStatus{
			model.CompleteOperationStatus,
			model.ErrorOperationStatus,
			model.CancelledOperationStatus,
		}).
		Where("scheduled_at < ? OR scheduled_at IS NULL", time.Now()).
		Preload("Operations", "status NOT IN (?)", []model.OperationStatus{
			model.CompleteOperationStatus,
			model.ErrorOperationStatus,
			model.CancelledOperationStatus,
		}).
		Order("COALESCE(scheduled_at, created_at) DESC").
		Where(&filter).
		First(&group).Error

	if err != nil {
		if db.IsRecordNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &group, nil
}
