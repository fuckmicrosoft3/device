package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Context keys for OTA operations
const (
	OTASessionContextKey contextKey = "ota_session"
	OTABatchContextKey   contextKey = "ota_batch"
)

// OTASessionAuth middleware validates and loads an OTA update session
func OTASessionAuth(repo repository.Repository, log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get session ID from URL parameter
		sessionID := c.Param("id")
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "OTA session ID is required",
			})
			c.Abort()
			return
		}

		// Get the OTA session from repository
		// This assumes an OTARepository interface that extends Repository
		otaRepo, ok := repo.(interface {
			FindOTASessionByID(ctx context.Context, sessionID string) (*models.OTAUpdateSession, error)
		})
		
		if !ok {
			log.Error("Repository does not implement OTA repository interface")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
			})
			c.Abort()
			return
		}

		session, err := otaRepo.FindOTASessionByID(c.Request.Context(), sessionID)
		if err != nil {
			log.WithError(err).Warn("OTA session not found")
			c.JSON(http.StatusNotFound, gin.H{
				"error": "OTA session not found",
			})
			c.Abort()
			return
		}

		// Store session in context
		c.Set(string(OTASessionContextKey), session)
		
		// Also set the device context if not already set
		if _, exists := c.Get(string(DeviceContextKey)); !exists {
			c.Set(string(DeviceContextKey), session.Device)
		}
		
		c.Next()
	}
}

// OTABatchAuth middleware validates and loads an OTA update batch
func OTABatchAuth(repo repository.Repository, log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get batch ID from URL parameter
		batchID := c.Param("batchId")
		if batchID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "OTA batch ID is required",
			})
			c.Abort()
			return
		}

		// Get the OTA batch from repository
		otaRepo, ok := repo.(interface {
			FindOTABatchByID(ctx context.Context, batchID string) (*models.OTAUpdateBatch, error)
		})
		
		if !ok {
			log.Error("Repository does not implement OTA repository interface")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
			})
			c.Abort()
			return
		}

		batch, err := otaRepo.FindOTABatchByID(c.Request.Context(), batchID)
		if err != nil {
			log.WithError(err).Warn("OTA batch not found")
			c.JSON(http.StatusNotFound, gin.H{
				"error": "OTA batch not found",
			})
			c.Abort()
			return
		}

		// Store batch in context
		c.Set(string(OTABatchContextKey), batch)
		c.Next()
	}
}

// OTASessionRateLimit middleware controls the rate of OTA update requests
func OTASessionRateLimit(log *logrus.Logger, requestsPerMinute int) gin.HandlerFunc {
	// Simple in-memory rate limiter - in production, use Redis or similar
	type deviceLimit struct {
		count    int
		lastReset time.Time
	}
	
	limits := make(map[uint]*deviceLimit)
	
	return func(c *gin.Context) {
		// Get device from context
		deviceVal, exists := c.Get(string(DeviceContextKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Device authentication required",
			})
			c.Abort()
			return
		}

		device, ok := deviceVal.(*models.Device)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid device in context",
			})
			c.Abort()
			return
		}

		// Check rate limit
		now := time.Now()
		limit, exists := limits[device.ID]
		if !exists || now.Sub(limit.lastReset) > time.Minute {
			// Reset limit for this device
			limits[device.ID] = &deviceLimit{
				count:     1,
				lastReset: now,
			}
		} else {
			// Increment count
			limit.count++
			
			// Check if limit exceeded
			if limit.count > requestsPerMinute {
				log.Warnf("Rate limit exceeded for device: %s", device.UID)
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "Rate limit exceeded",
					"code":  "rate_limit_exceeded",
					"retry_after": int(60 - now.Sub(limit.lastReset).Seconds()),
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// OTAChunkValidator validates chunk download parameters
func OTAChunkValidator(log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get chunk parameters
		offsetStr := c.Query("offset")
		sizeStr := c.Query("size")
		
		// Validate offset
		offset, err := strconv.ParseUint(offsetStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid offset parameter",
			})
			c.Abort()
			return
		}
		
		// Validate size
		size, err := strconv.ParseUint(sizeStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid size parameter",
			})
			c.Abort()
			return
		}
		
		// Get OTA session from context
		sessionVal, exists := c.Get(string(OTASessionContextKey))
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "OTA session not found in context",
			})
			c.Abort()
			return
		}
		
		session, ok := sessionVal.(*models.OTAUpdateSession)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid OTA session in context",
			})
			c.Abort()
			return
		}
		
		// Check if offset is valid
		if offset > session.TotalBytes {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Offset exceeds firmware size",
			})
			c.Abort()
			return
		}
		
		// Check if size is valid
		if offset + size > session.TotalBytes {
			// Adjust size to valid range
			size = session.TotalBytes - offset
			// Store adjusted size for handler
			c.Set("adjusted_chunk_size", size)
		}
		
		// Store validated chunk parameters in context
		c.Set("chunk_offset", offset)
		c.Set("chunk_size", size)
		
		c.Next()
	}
}

// GetOTASessionFromContext retrieves an OTA update session from the context
func GetOTASessionFromContext(c *gin.Context) (*models.OTAUpdateSession, error) {
	sessionVal, exists := c.Get(string(OTASessionContextKey))
	if !exists {
		return nil, fmt.Errorf("OTA session not found in context")
	}

	session, ok := sessionVal.(*models.OTAUpdateSession)
	if !ok {
		return nil, fmt.Errorf("OTA session in context has incorrect type")
	}

	return session, nil
}

// GetOTABatchFromContext retrieves an OTA update batch from the context
func GetOTABatchFromContext(c *gin.Context) (*models.OTAUpdateBatch, error) {
	batchVal, exists := c.Get(string(OTABatchContextKey))
	if !exists {
		return nil, fmt.Errorf("OTA batch not found in context")
	}

	batch, ok := batchVal.(*models.OTAUpdateBatch)
	if !ok {
		return nil, fmt.Errorf("OTA batch in context has incorrect type")
	}

	return batch, nil
}