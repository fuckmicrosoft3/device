package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/repository"
	"example.com/backstage/services/device/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Context keys for firmware operations
const (
	FirmwareReleaseContextKey contextKey = "firmware_release"
	FirmwareSignatureKey      contextKey = "firmware_signature"
)

// FirmwareReleaseAuth middleware validates firmware release access and permissions
func FirmwareReleaseAuth(repo repository.Repository, log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get release ID from URL parameter
		releaseIDStr := c.Param("id")
		if releaseIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Firmware release ID is required",
			})
			c.Abort()
			return
		}

		// Convert to uint
		var releaseID uint
		if _, err := fmt.Sscanf(releaseIDStr, "%d", &releaseID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid firmware release ID format",
			})
			c.Abort()
			return
		}

		// Get the firmware release
		release, err := repo.FindFirmwareReleaseByID(c.Request.Context(), releaseID)
		if err != nil {
			log.WithError(err).Warn("Firmware release not found")
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Firmware release not found",
			})
			c.Abort()
			return
		}

		// Store the release in context
		c.Set(string(FirmwareReleaseContextKey), release)
		c.Next()
	}
}

// FirmwareSignatureVerification validates firmware signatures for OTA operations
func FirmwareSignatureVerification(repo repository.Repository, log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get signature from header
		signature := c.GetHeader("X-Firmware-Signature")
		if signature == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Firmware signature is required",
			})
			c.Abort()
			return
		}

		// Get the firmware release from context if already present
		var release *models.FirmwareRelease
		releaseVal, exists := c.Get(string(FirmwareReleaseContextKey))
		if exists {
			var ok bool
			release, ok = releaseVal.(*models.FirmwareRelease)
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Invalid firmware release in context",
				})
				c.Abort()
				return
			}
		} else {
			// Get release ID from URL parameter
			releaseIDStr := c.Param("id")
			if releaseIDStr == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "Firmware release ID is required",
				})
				c.Abort()
				return
			}

			// Convert to uint
			var releaseID uint
			if _, err := fmt.Sscanf(releaseIDStr, "%d", &releaseID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "Invalid firmware release ID format",
				})
				c.Abort()
				return
			}

			// Get the firmware release
			var err error
			release, err = repo.FindFirmwareReleaseByID(c.Request.Context(), releaseID)
			if err != nil {
				log.WithError(err).Warn("Firmware release not found")
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Firmware release not found",
				})
				c.Abort()
				return
			}

			// Store the release in context
			c.Set(string(FirmwareReleaseContextKey), release)
		}

		// Verify signature (sample implementation)
		// In a real implementation, this would use the utils.VerifySignature function
		isValid := utils.VerifySignature(signature, release.FileHash)
		if !isValid {
			log.Warn("Invalid firmware signature")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid firmware signature",
			})
			c.Abort()
			return
		}

		// Store signature in context
		c.Set(string(FirmwareSignatureKey), signature)
		c.Next()
	}
}

// DeviceOTAAuth middleware validates device eligibility for OTA updates
func DeviceOTAAuth(repo repository.Repository, log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First get the device from the standard DeviceAuth middleware
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

		// Check if device is active
		if !device.Active {
			log.Warnf("Inactive device attempting OTA: %s", device.UID)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Device is inactive",
				"code":  "device_inactive",
			})
			c.Abort()
			return
		}

		// Check if device has updates allowed
		if !device.AllowUpdates {
			log.Warnf("Device with updates disabled attempting OTA: %s", device.UID)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Updates are disabled for this device",
				"code":  "updates_disabled",
			})
			c.Abort()
			return
		}

		// Check if there's an ongoing update session for this device
		// This would involve checking the OTA update repository
		// For now, we'll leave this as a placeholder
		// otaRepo := repo.(repository.OTARepository)
		// hasActiveSession, err := otaRepo.HasActiveSession(c.Request.Context(), device.ID)
		// if err != nil {
		//     log.WithError(err).Error("Failed to check active OTA sessions")
		//     c.JSON(http.StatusInternalServerError, gin.H{
		//         "error": "Failed to check active OTA sessions",
		//     })
		//     c.Abort()
		//     return
		// }
		//
		// if hasActiveSession {
		//     log.Warnf("Device already has an active OTA session: %s", device.UID)
		//     c.JSON(http.StatusConflict, gin.H{
		//         "error": "Device already has an active update session",
		//         "code":  "active_session_exists",
		//     })
		//     c.Abort()
		//     return
		// }

		c.Next()
	}
}

// GetFirmwareReleaseFromContext retrieves a firmware release from the context
func GetFirmwareReleaseFromContext(c *gin.Context) (*models.FirmwareRelease, error) {
	releaseVal, exists := c.Get(string(FirmwareReleaseContextKey))
	if !exists {
		return nil, fmt.Errorf("firmware release not found in context")
	}

	release, ok := releaseVal.(*models.FirmwareRelease)
	if !ok {
		return nil, fmt.Errorf("firmware release in context has incorrect type")
	}

	return release, nil
}