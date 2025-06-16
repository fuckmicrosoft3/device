package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
	
	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/service"
	
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/google/uuid"
)

// OTAHandler handles over-the-air update operations
type OTAHandler struct {
	service service.Service
	log     *logrus.Logger
}

// NewOTAHandler creates a new OTAHandler instance
func NewOTAHandler(svc service.Service, log *logrus.Logger) *OTAHandler {
	return &OTAHandler{
		service: svc,
		log:     log,
	}
}

// CreateUpdateSession creates a new OTA update session for a device
func (h *OTAHandler) CreateUpdateSession(c *gin.Context) {
	var request struct {
		DeviceID        uint   `json:"device_id" binding:"required"`
		FirmwareID      uint   `json:"firmware_id" binding:"required"`
		ScheduledAt     string `json:"scheduled_at"`
		Priority        uint   `json:"priority"`
		ForceUpdate     bool   `json:"force_update"`
		AllowRollback   bool   `json:"allow_rollback"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		h.log.WithError(err).Warn("Invalid update session format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid update session format",
		})
		return
	}
	
	// Parse scheduled time if provided, otherwise use current time
	var scheduledAt time.Time
	if request.ScheduledAt != "" {
		var err error
		scheduledAt, err = time.Parse(time.RFC3339, request.ScheduledAt)
		if err != nil {
			h.log.WithError(err).Warn("Invalid scheduled time format")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid scheduled time format. Use ISO 8601 / RFC 3339 format (e.g., 2025-04-02T15:04:05Z)",
			})
			return
		}
	} else {
		scheduledAt = time.Now()
	}
	
	// Set default priority if not provided
	if request.Priority == 0 {
		request.Priority = 5 // Default priority (1-10, 1 being highest)
	}
	
	// Create the session
	session := &models.OTAUpdateSession{
		SessionID:          uuid.New().String(),
		DeviceID:           request.DeviceID,
		FirmwareReleaseID:  request.FirmwareID,
		Status:             models.OTAStatusScheduled,
		ScheduledAt:        scheduledAt,
		Priority:           request.Priority,
		ForceUpdate:        request.ForceUpdate,
		AllowRollback:      request.AllowRollback,
	}
	
	if err := h.service.CreateOTAUpdateSession(c, session); err != nil {
		h.log.WithError(err).Error("Failed to create update session")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create update session",
		})
		return
	}
	
	c.JSON(http.StatusCreated, session)
}

// CreateBatchUpdate creates update sessions for multiple devices
func (h *OTAHandler) CreateBatchUpdate(c *gin.Context) {
	var request struct {
		DeviceIDs       []uint `json:"device_ids" binding:"required"`
		FirmwareID      uint   `json:"firmware_id" binding:"required"`
		ScheduledAt     string `json:"scheduled_at"`
		Priority        uint   `json:"priority"`
		ForceUpdate     bool   `json:"force_update"`
		AllowRollback   bool   `json:"allow_rollback"`
		MaxConcurrent   uint   `json:"max_concurrent"`
		Notes           string `json:"notes"`
		OrganizationID  *uint  `json:"organization_id"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		h.log.WithError(err).Warn("Invalid batch update format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid batch update format",
		})
		return
	}
	
	// Parse scheduled time if provided, otherwise use current time
	var scheduledAt time.Time
	if request.ScheduledAt != "" {
		var err error
		scheduledAt, err = time.Parse(time.RFC3339, request.ScheduledAt)
		if err != nil {
			h.log.WithError(err).Warn("Invalid scheduled time format")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid scheduled time format. Use ISO 8601 / RFC 3339 format (e.g., 2025-04-02T15:04:05Z)",
			})
			return
		}
	} else {
		scheduledAt = time.Now()
	}
	
	// Set default values if not provided
	if request.Priority == 0 {
		request.Priority = 5
	}
	
	if request.MaxConcurrent == 0 {
		request.MaxConcurrent = 100
	}
	
	batch := &models.OTAUpdateBatch{
		BatchID:           uuid.New().String(),
		FirmwareReleaseID: request.FirmwareID,
		Status:            models.OTAStatusScheduled,
		ScheduledAt:       scheduledAt,
		Priority:          request.Priority,
		ForceUpdate:       request.ForceUpdate,
		AllowRollback:     request.AllowRollback,
		TotalCount:        uint(len(request.DeviceIDs)),
		PendingCount:      uint(len(request.DeviceIDs)),
		CreatedBy:         c.GetString("username"),
		Notes:             request.Notes,
		OrganizationID:    request.OrganizationID,
		MaxConcurrent:     request.MaxConcurrent,
	}
	
	// Create the batch update
	result, err := h.service.CreateOTAUpdateBatch(c, batch, request.DeviceIDs)
	if err != nil {
		h.log.WithError(err).Error("Failed to create batch update")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create batch update",
		})
		return
	}
	
	c.JSON(http.StatusCreated, result)
}

// DeviceCheckUpdate handles a device checking for available updates
func (h *OTAHandler) DeviceCheckUpdate(c *gin.Context) {
	deviceUID := c.Param("uid")
	if deviceUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Device UID is required",
		})
		return
	}
	
	// Get current version from query parameters
	currentVersion := c.DefaultQuery("version", "")
	
	// Check for available updates
	update, err := h.service.CheckDeviceUpdate(c, deviceUID, currentVersion)
	if err != nil {
		h.log.WithError(err).Error("Failed to check for updates")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to check for updates",
		})
		return
	}
	
	if update == nil {
		// No update available
		c.JSON(http.StatusOK, gin.H{
			"update_available": false,
		})
		return
	}
	
	// Update available
	c.JSON(http.StatusOK, gin.H{
		"update_available": true,
		"session_id":       update.SessionID,
		"firmware_id":      update.FirmwareReleaseID,
		"version":          update.FirmwareRelease.Version,
		"size":             update.FirmwareRelease.Size,
		"checksum":         update.FirmwareRelease.FileHash,
		"force_update":     update.ForceUpdate,
	})
}

// AcknowledgeUpdate handles a device acknowledging an update
func (h *OTAHandler) AcknowledgeUpdate(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Session ID is required",
		})
		return
	}
	
	var request struct {
		DeviceVersion string `json:"device_version"`
		Accept        bool   `json:"accept"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		h.log.WithError(err).Warn("Invalid acknowledgment format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid acknowledgment format",
		})
		return
	}
	
	// Update session status
	if err := h.service.AcknowledgeUpdate(c, sessionID, request.DeviceVersion, request.Accept); err != nil {
		h.log.WithError(err).Error("Failed to acknowledge update")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to acknowledge update",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"id":     sessionID,
	})
}

// ChunkedDownload handles a device downloading firmware in chunks
func (h *OTAHandler) ChunkedDownload(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Session ID is required",
		})
		return
	}
	
	// Get chunk information from query parameters
	chunkIndexStr := c.DefaultQuery("chunk", "0")
	chunkSizeStr := c.DefaultQuery("size", "32768") // Default 32KB
	
	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid chunk index",
		})
		return
	}
	
	chunkSize, err := strconv.Atoi(chunkSizeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid chunk size",
		})
		return
	}
	
	// Limit chunk size to prevent excessive resource usage
	if chunkSize > 1024*1024 {
		chunkSize = 1024 * 1024 // Max 1MB per chunk
	}
	
	// Get session and firmware information
	session, err := h.service.GetOTAUpdateSession(c, sessionID)
	if err != nil {
		h.log.WithError(err).Error("Failed to get update session")
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Update session not found",
		})
		return
	}
	
	// Validate session status
	if session.Status != models.OTAStatusAcknowledged && 
	   session.Status != models.OTAStatusDownloading {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Update not in valid state for download",
			"status": string(session.Status),
		})
		return
	}
	
	// Get firmware file
	firmware, err := h.service.GetFirmwareRelease(c, session.FirmwareReleaseID)
	if err != nil {
		h.log.WithError(err).Error("Failed to get firmware")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get firmware",
		})
		return
	}
	
	// Open the firmware file
	file, err := os.Open(firmware.FilePath)
	if err != nil {
		h.log.WithError(err).Error("Failed to open firmware file")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to open firmware file",
		})
		return
	}
	defer file.Close()
	
	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		h.log.WithError(err).Error("Failed to get file info")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get file info",
		})
		return
	}
	
	// Calculate total chunks
	totalSize := fileInfo.Size()
	totalChunks := (int(totalSize) + chunkSize - 1) / chunkSize
	
	// Validate chunk index
	if chunkIndex >= totalChunks {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Chunk index out of range",
			"total_chunks": totalChunks,
		})
		return
	}
	
	// Calculate offset and read size
	offset := int64(chunkIndex * chunkSize)
	readSize := chunkSize
	
	// Adjust read size for the last chunk
	if offset+int64(readSize) > totalSize {
		readSize = int(totalSize - offset)
	}
	
	// Seek to the correct position
	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		h.log.WithError(err).Error("Failed to seek in file")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to seek in file",
		})
		return
	}
	 
	// Buffer for the chunk
	buffer := make([]byte, readSize)
	bytesRead, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		h.log.WithError(err).Error("Failed to read file chunk")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read file chunk",
		})
		return
	}
	
	// Update session stats
	go func() {
		// Use a new context since the request context will be cancelled
		ctx := context.Background()
		
		// If this is the first chunk, update status
		if chunkIndex == 0 {
			if err := h.service.UpdateOTASessionStatus(ctx, sessionID, models.OTAStatusDownloading); err != nil {
				h.log.WithError(err).Error("Failed to update session status")
			}
		}
		
		// Calculate download progress
		bytesDownloaded := int64(chunkIndex * chunkSize)
		if chunkIndex == totalChunks-1 {
			bytesDownloaded = totalSize
		} else {
			bytesDownloaded += int64(bytesRead)
		}
		
		// Update session with download progress
		if err := h.service.UpdateOTADownloadProgress(ctx, sessionID, uint64(bytesDownloaded), uint(chunkIndex), uint(totalChunks)); err != nil {
			h.log.WithError(err).Error("Failed to update download progress")
		}
	}()
	
	// Set response headers
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", bytesRead))
	c.Header("X-Chunk-Index", fmt.Sprintf("%d", chunkIndex))
	c.Header("X-Total-Chunks", fmt.Sprintf("%d", totalChunks))
	c.Header("X-Chunk-Size", fmt.Sprintf("%d", bytesRead))
	c.Header("X-Total-Size", fmt.Sprintf("%d", totalSize))
	
	// Send the chunk data
	c.Data(http.StatusOK, "application/octet-stream", buffer[:bytesRead])
}

// DownloadComplete handles a device signaling that download is complete
func (h *OTAHandler) DownloadComplete(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Session ID is required",
		})
		return
	}
	
	var request struct {
		Checksum string `json:"checksum"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		h.log.WithError(err).Warn("Invalid download complete format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid download complete format",
		})
		return
	}
	
	// Update session status and verify checksum
	if err := h.service.CompleteOTADownload(c, sessionID, request.Checksum); err != nil {
		h.log.WithError(err).Error("Failed to complete download")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to complete download: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"id":     sessionID,
	})
}

// FlashComplete handles a device signaling that firmware flashing is complete
func (h *OTAHandler) FlashComplete(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Session ID is required",
		})
		return
	}
	
	var request struct {
		Success      bool   `json:"success"`
		ErrorMessage string `json:"error_message"`
		NewVersion   string `json:"new_version"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		h.log.WithError(err).Warn("Invalid flash complete format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid flash complete format",
		})
		return
	}
	
	// Update session status
	if err := h.service.CompleteOTAFlash(c, sessionID, request.Success, request.ErrorMessage, request.NewVersion); err != nil {
		h.log.WithError(err).Error("Failed to complete flash")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to complete flash: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"id":     sessionID,
	})
}

// GetUpdateSession retrieves information about an update session
func (h *OTAHandler) GetUpdateSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Session ID is required",
		})
		return
	}
	
	session, err := h.service.GetOTAUpdateSession(c, sessionID)
	if err != nil {
		h.log.WithError(err).Error("Failed to get update session")
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Update session not found",
		})
		return
	}
	
	c.JSON(http.StatusOK, session)
}

// ListUpdateSessions lists all update sessions with filtering options
func (h *OTAHandler) ListUpdateSessions(c *gin.Context) {
	// Get filter parameters
	deviceIDStr := c.DefaultQuery("device_id", "")
	batchID := c.DefaultQuery("batch_id", "")
	status := c.DefaultQuery("status", "")
	
	var deviceID *uint
	if deviceIDStr != "" {
		id, err := strconv.ParseUint(deviceIDStr, 10, 64)
		if err == nil {
			uintID := uint(id)
			deviceID = &uintID
		}
	}
	
	sessions, err := h.service.ListOTAUpdateSessions(c, deviceID, batchID, models.OTAUpdateStatus(status))
	if err != nil {
		h.log.WithError(err).Error("Failed to list update sessions")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list update sessions",
		})
		return
	}
	
	c.JSON(http.StatusOK, sessions)
}

// CancelUpdateSession cancels an in-progress update session
func (h *OTAHandler) CancelUpdateSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Session ID is required",
		})
		return
	}
	
	if err := h.service.CancelOTAUpdateSession(c, sessionID); err != nil {
		h.log.WithError(err).Error("Failed to cancel update session")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to cancel update session: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"id":     sessionID,
	})
}