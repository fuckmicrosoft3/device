// services/device/internal/core/errors.go
package core

import (
	"errors"
	"fmt"
)

// Business errors.
var (
	// Device errors.
	ErrDeviceNotFound      = errors.New("device not found")
	ErrDeviceAlreadyExists = errors.New("device already exists")
	ErrDeviceInactive      = errors.New("device is inactive")
	ErrDeviceUpdatesDenied = errors.New("device updates are disabled")

	// Organization errors.
	ErrOrganizationNotFound = errors.New("organization not found")
	ErrOrganizationInactive = errors.New("organization is inactive")

	// Firmware errors.
	ErrFirmwareNotFound  = errors.New("firmware release not found")
	ErrFirmwareNotActive = errors.New("firmware release not active")

	// Update errors.
	ErrUpdateInProgress = errors.New("update already in progress")
	ErrChecksumMismatch = errors.New("checksum verification failed")
	ErrVersionDowngrade = errors.New("version downgrade not allowed")
)

// BusinessError represents a business logic error with a code.
type BusinessError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e BusinessError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
