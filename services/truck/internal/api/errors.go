package api

import (
	"errors"
	"net/http"

	"github.com/sirupsen/logrus"
)

// ErrorResponse defines the structure of an error response
type ErrorResponse struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// Error represents an API error
type Error struct {
	Message    string
	StatusCode int
	Code       string
}

// Error implements the error interface
func (e *Error) Error() string {
	return e.Message
}

// Common API errors
var (
	ErrInvalidRequest     = &Error{Message: "Invalid request", StatusCode: http.StatusBadRequest, Code: "INVALID_REQUEST"}
	ErrNotFound           = &Error{Message: "Resource not found", StatusCode: http.StatusNotFound, Code: "NOT_FOUND"}
	ErrInternalServer     = &Error{Message: "Internal server error", StatusCode: http.StatusInternalServerError, Code: "INTERNAL_ERROR"}
	ErrUnauthorized       = &Error{Message: "Unauthorized", StatusCode: http.StatusUnauthorized, Code: "UNAUTHORIZED"}
	ErrForbidden          = &Error{Message: "Forbidden", StatusCode: http.StatusForbidden, Code: "FORBIDDEN"}
	ErrConflict           = &Error{Message: "Resource already exists", StatusCode: http.StatusConflict, Code: "CONFLICT"}
	ErrValidation         = &Error{Message: "Validation error", StatusCode: http.StatusBadRequest, Code: "VALIDATION_ERROR"}
	ErrTooManyRequests    = &Error{Message: "Too many requests", StatusCode: http.StatusTooManyRequests, Code: "TOO_MANY_REQUESTS"}
	ErrServiceUnavailable = &Error{Message: "Service unavailable", StatusCode: http.StatusServiceUnavailable, Code: "SERVICE_UNAVAILABLE"}
)

// WriteError writes an error response
func WriteError(w http.ResponseWriter, err error) {
	var apiError *Error
	if errors.As(err, &apiError) {
		writeJSONResponse(w, apiError.StatusCode, ErrorResponse{
			Message: apiError.Message,
			Code:    apiError.Code,
		})
		return
	}

	// Log unknown errors
	logrus.WithError(err).Error("Unhandled error")
	writeJSONResponse(w, http.StatusInternalServerError, ErrorResponse{
		Message: "Internal server error",
		Code:    "INTERNAL_ERROR",
	})
}

// NewValidationError creates a new validation error with a custom message
func NewValidationError(message string) *Error {
	return &Error{
		Message:    message,
		StatusCode: http.StatusBadRequest,
		Code:       "VALIDATION_ERROR",
	}
}

// NewError creates a new API error with custom details
func NewError(message string, statusCode int, code string) *Error {
	return &Error{
		Message:    message,
		StatusCode: statusCode,
		Code:       code,
	}
}
