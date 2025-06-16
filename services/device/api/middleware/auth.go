// api/middleware/auth.go
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// contextKey is a type for context keys
type contextKey string

// Context keys
const (
	APIKeyContextKey contextKey = "api_key"
	DeviceContextKey contextKey = "device"
)

// APIKeyAuth middleware validates API tokens from Authorization header
func APIKeyAuth(repo repository.Repository, log *logrus.Logger, requiredLevel models.AuthorizationLevel) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Check if Authorization header is present
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}

		// Extract token from header
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid Authorization header format. Expected: 'Bearer {token}'",
			})
			c.Abort()
			return
		}

		token := parts[1]

		// Validate token
		apiKey, err := repo.GetAPIKeyByKey(c.Request.Context(), token)
		if err != nil {
			log.WithError(err).Warn("Invalid API key")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
			})
			c.Abort()
			return
		}

		// Check if key is expired
		if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
			log.Warn("Expired API key")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "API key expired",
			})
			c.Abort()
			return
		}

		// Check if key has sufficient permissions
		if apiKey.AuthorizationLevel < requiredLevel {
			log.Warnf("Insufficient permissions. Required: %d, Provided: %d", 
				requiredLevel, apiKey.AuthorizationLevel)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
			})
			c.Abort()
			return
		}

		// Update last used timestamp
		now := time.Now()
		apiKey.LastUsedAt = &now
		go func() {
			// Update in a goroutine to avoid blocking the request
			repo.UpdateAPIKey(context.Background(), apiKey)
		}()

		// Store API key in context for later use if needed
		c.Set(string(APIKeyContextKey), apiKey)

		c.Next()
	}
}

// DeviceAuth middleware validates that the device exists and is accessible
func DeviceAuth(repo repository.Repository, log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract device UID from URL parameter
		deviceUID := c.Param("deviceUID")
		if deviceUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Device UID is required",
			})
			c.Abort()
			return
		}

		// Get the device from the database
		device, err := repo.FindDeviceByUID(c.Request.Context(), deviceUID)
		if err != nil {
			log.WithError(err).Warn("Device not found")
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Device not found",
			})
			c.Abort()
			return
		}
		
		// Check if the device is active
		if !device.Active {
			log.Warnf("Inactive device attempted to authenticate: %s", deviceUID)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Device is inactive",
			})
			c.Abort()
			return
		}

		// Store the device in the context for later use
		c.Set(string(DeviceContextKey), device)

		c.Next()
	}
}

// GetDeviceFromContext retrieves a device from the context
func GetDeviceFromContext(c *gin.Context) (*models.Device, error) {
	deviceVal, exists := c.Get(string(DeviceContextKey))
	if !exists {
		return nil, errors.New("device not found in context")
	}

	device, ok := deviceVal.(*models.Device)
	if !ok {
		return nil, errors.New("device in context has incorrect type")
	}

	return device, nil
}