package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"example.com/backstage/services/truck/internal/db"
	"example.com/backstage/services/truck/internal/model"
)

// OperationRepository defines the interface for operation repository
type OperationRepository interface {
	Create(ctx context.Context, operation *model.Operation) (*model.Operation, error)
	Update(ctx context.Context, operation *model.Operation) (*model.Operation, error)
	GetByID(ctx context.Context, id string) (*model.Operation, error)
	GetBy(ctx context.Context, filter model.Operation) (*model.Operation, error)
	GetActiveBy(ctx context.Context, filter model.Operation) (*model.Operation, error)
	FindAllActiveBy(ctx context.Context, filter model.Operation) ([]*model.Operation, error)
	GetOperationSessionByID(ctx context.Context, id string) (*model.OperationSession, error)
	CreateOperationSession(ctx context.Context, session *model.OperationSession) (*model.OperationSession, error)
	FindOperationSessionsBy(ctx context.Context, filter model.OperationSession, start, end *time.Time) ([]*model.OperationSession, error)
}

// operationRepository implements OperationRepository
type operationRepository struct {
	db *gorm.DB
}

// NewOperationRepository creates a new operation repository
func NewOperationRepository(db *gorm.DB) OperationRepository {
	return &operationRepository{db: db}
}

// Create creates a new operation
func (r *operationRepository) Create(ctx context.Context, operation *model.Operation) (*model.Operation, error) {
	if err := r.db.WithContext(ctx).Create(operation).Error; err != nil {
		return nil, err
	}
	return operation, nil
}

// Update updates an operation
func (r *operationRepository) Update(ctx context.Context, operation *model.Operation) (*model.Operation, error) {
	if err := r.db.WithContext(ctx).Updates(operation).Error; err != nil {
		return nil, err
	}
	return operation, nil
}

// GetByID gets an operation by ID
func (r *operationRepository) GetByID(ctx context.Context, id string) (*model.Operation, error) {
	var operation model.Operation
	err := r.db.WithContext(ctx).Where("uuid = ?", id).First(&operation).Error
	if err != nil {
		if db.IsRecordNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &operation, nil
}

// GetBy gets an operation by filter
func (r *operationRepository) GetBy(ctx context.Context, filter model.Operation) (*model.Operation, error) {
	var operation model.Operation
	err := r.db.WithContext(ctx).Where(&filter).First(&operation).Error
	if err != nil {
		if db.IsRecordNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &operation, nil
}

// GetActiveBy gets an active operation by filter
func (r *operationRepository) GetActiveBy(ctx context.Context, filter model.Operation) (*model.Operation, error) {
	var operation model.Operation
	err := r.db.WithContext(ctx).
		Where("status NOT IN (?)", []model.OperationStatus{
			model.CompleteOperationStatus,
			model.ErrorOperationStatus,
			model.CancelledOperationStatus,
		}).
		Where(&filter).
		Order("created_at DESC").
		First(&operation).Error

	if err != nil {
		if db.IsRecordNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &operation, nil
}

// FindAllActiveBy finds all active operations by filter
func (r *operationRepository) FindAllActiveBy(ctx context.Context, filter model.Operation) ([]*model.Operation, error) {
	var operations []*model.Operation
	err := r.db.WithContext(ctx).
		Where("status NOT IN (?)", []model.OperationStatus{
			model.CompleteOperationStatus,
			model.ErrorOperationStatus,
			model.CancelledOperationStatus,
		}).
		Where(&filter).
		Order("created_at DESC").
		Find(&operations).Error

	if err != nil {
		return nil, err
	}
	return operations, nil
}

// GetOperationSessionByID gets an operation session by ID
func (r *operationRepository) GetOperationSessionByID(ctx context.Context, id string) (*model.OperationSession, error) {
	var session model.OperationSession
	err := r.db.WithContext(ctx).
		Preload("Operation").
		Preload("OperationGroup").
		Where("uuid = ?", id).
		First(&session).Error

	if err != nil {
		if db.IsRecordNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &session, nil
}

// CreateOperationSession creates a new operation session
func (r *operationRepository) CreateOperationSession(ctx context.Context, session *model.OperationSession) (*model.OperationSession, error) {
	if err := r.db.WithContext(ctx).Omit("Operation", "OperationGroup").Create(session).Error; err != nil {
		return nil, err
	}
	return session, nil
}

// FindOperationSessionsBy finds operation sessions by filter and date range
func (r *operationRepository) FindOperationSessionsBy(ctx context.Context, filter model.OperationSession, start, end *time.Time) ([]*model.OperationSession, error) {
	var sessions []*model.OperationSession
	
	query := r.db.WithContext(ctx).
		Preload("Operation").
		Preload("OperationGroup").
		Where(&filter).
		Order("complete, created_at")

	if start != nil {
		query = query.Where("started_at >= ?", start)
	}

	if end != nil {
		query = query.Where("completed_at <= ? OR completed_at IS NULL", end)
	}

	if err := query.Find(&sessions).Error; err != nil {
		return nil, err
	}

	return sessions, nil
}