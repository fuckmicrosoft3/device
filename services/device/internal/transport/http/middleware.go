package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"example.com/backstage/services/device/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Logger middleware.
func Logger(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		logger.WithFields(logrus.Fields{
			"status":    statusCode,
			"latency":   latency,
			"client_ip": c.ClientIP(),
			"method":    c.Request.Method,
			"path":      path,
		}).Info("Request processed")
	}
}

// APIKeyAuth middleware.
func APIKeyAuth(repo core.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			c.Abort()
			return
		}

		apiKey, err := repo.GetAPIKey(c.Request.Context(), parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			c.Abort()
			return
		}

		// Check expiration
		if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "api key expired"})
			c.Abort()
			return
		}

		// Update last used
		go repo.UpdateAPIKeyLastUsed(context.Background(), apiKey.Key)

		c.Set("api_key", apiKey)
		c.Next()
	}
}

// RequirePermission checks if API key has required permission.
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKeyVal, exists := c.Get("api_key")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no api key found"})
			c.Abort()
			return
		}

		apiKey, ok := apiKeyVal.(*core.APIKey)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid api key type"})
			c.Abort()
			return
		}

		// Check if permission exists
		hasPermission := false
		for _, perm := range apiKey.Permissions {
			if perm == permission || perm == "admin" {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// DeviceAuth middleware for device-specific endpoints.
func DeviceAuth(repo core.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceUID := c.Param("uid")
		if deviceUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "device uid required"})
			c.Abort()
			return
		}

		device, err := repo.GetDeviceByUID(c.Request.Context(), deviceUID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
			c.Abort()
			return
		}

		if !device.Active {
			c.JSON(http.StatusForbidden, gin.H{"error": "device inactive"})
			c.Abort()
			return
		}

		c.Set("device", device)
		c.Next()
	}
}

// ErrorHandler middleware.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			c.JSON(c.Writer.Status(), gin.H{
				"error": err.Error(),
			})
		}
	}
}

// CORS middleware.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
