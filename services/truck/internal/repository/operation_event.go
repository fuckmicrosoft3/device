package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go.novek.io/truck/internal/model"
)

// OperationEventRepository defines the interface for operation event repository
type OperationEventRepository interface {
	Create(ctx context.Context, event *model.OperationEvent) error
}

// operationEventRepository implements OperationEventRepository
type operationEventRepository struct {
	db *gorm.DB
}

// NewOperationEventRepository creates a new operation event repository
func NewOperationEventRepository(db *gorm.DB) OperationEventRepository {
	return &operationEventRepository{db: db}
}

// Create creates a new operation event
func (r *operationEventRepository) Create(ctx context.Context, event *model.OperationEvent) error {
	return r.db.WithContext(ctx).Omit(clause.Associations).Create(event).Error
}
