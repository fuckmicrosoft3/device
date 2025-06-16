package repository

import "errors"

// Common repository errors
var (
	ErrNotFound    = errors.New("record not found")
	ErrCreateFailed = errors.New("failed to create record")
	ErrUpdateFailed = errors.New("failed to update record")
	ErrDeleteFailed = errors.New("failed to delete record")
	ErrDuplicateKey = errors.New("duplicate key violation")
)