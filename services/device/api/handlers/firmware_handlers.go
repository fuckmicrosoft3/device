package handlers

import (
	"net/http"
	"strconv"
	"fmt"
	
	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/service"
	"example.com/backstage/services/device/internal/utils"
	
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// FirmwareHandler handles firmware-related requests
type FirmwareHandler struct {
	service service.Service
	log     *logrus.Logger
}

// NewFirmwareHandler creates a new FirmwareHandler instance
func NewFirmwareHandler(svc service.Service, log *logrus.Logger) *FirmwareHandler {
	return &FirmwareHandler{
		service: svc,
		log:     log,
	}
}

// CreateFirmwareRelease handles firmware release creation
func (h *FirmwareHandler) CreateFirmwareRelease(c *gin.Context) {
	var release models.FirmwareRelease
	if err := c.ShouldBindJSON(&release); err != nil {
		h.log.WithError(err).Warn("Invalid firmware release format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid firmware release format",
		})
		return
	}
	
	if release.Version == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Version is required",
		})
		return
	}
	
	if release.FilePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "File path is required",
		})
		return
	}
	
	// Parse semantic version
	semVer, err := utils.ParseSemanticVersion(release.Version)
	if err != nil {
		h.log.WithError(err).Warn("Invalid semantic version format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid semantic version: %v", err),
		})
		return
	}
	
	// Create extended firmware release
	extendedRelease := &models.FirmwareReleaseExtended{
		FirmwareRelease: release,
		MajorVersion:    semVer.Major,
		MinorVersion:    semVer.Minor,
		PatchVersion:    semVer.Patch,
		PreReleaseVersion: semVer.PreRelease,
		BuildMetadata:   semVer.Build,
	}
	
	if err := h.service.CreateFirmwareRelease(c, &release); err != nil {
		h.log.WithError(err).Error("Failed to create firmware release")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create firmware release",
		})
		return
	}
	
	c.JSON(http.StatusCreated, release)
}

// GetFirmwareRelease handles firmware release retrieval
func (h *FirmwareHandler) GetFirmwareRelease(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid firmware release ID",
		})
		return
	}
	
	release, err := h.service.GetFirmwareRelease(c, uint(id))
	if err != nil {
		h.log.WithError(err).Error("Failed to get firmware release")
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Firmware release not found",
		})
		return
	}
	
	c.JSON(http.StatusOK, release)
}

// ListFirmwareReleases handles listing firmware releases
func (h *FirmwareHandler) ListFirmwareReleases(c *gin.Context) {
	releaseType := models.ReleaseType(c.DefaultQuery("type", ""))
	
	releases, err := h.service.ListFirmwareReleases(c, releaseType)
	if err != nil {
		h.log.WithError(err).Error("Failed to list firmware releases")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list firmware releases",
		})
		return
	}
	
	c.JSON(http.StatusOK, releases)
}

// ActivateFirmwareRelease handles activating a firmware release
func (h *FirmwareHandler) ActivateFirmwareRelease(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid firmware release ID",
		})
		return
	}
	
	if err := h.service.ActivateFirmwareRelease(c, uint(id)); err != nil {
		h.log.WithError(err).Error("Failed to activate firmware release")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to activate firmware release",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"id":     id,
	})
}

// VerifyFirmwareRelease handles verification of a firmware release
func (h *FirmwareHandler) VerifyFirmwareRelease(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid firmware release ID",
		})
		return
	}
	
	validation, err := h.service.VerifyFirmwareRelease(c, uint(id))
	if err != nil {
		h.log.WithError(err).Error("Failed to verify firmware release")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to verify firmware release: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusOK, validation)
}