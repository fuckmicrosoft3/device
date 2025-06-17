package api

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"example.com/backstage/services/device/internal/core"
	"github.com/gin-gonic/gin"
)

// APIHandlers holds all HTTP handlers
type APIHandlers struct {
	services *core.ServiceRegistry
}

// NewAPIHandlers creates a new handler instance
func NewAPIHandlers(services *core.ServiceRegistry) *APIHandlers {
	return &APIHandlers{services: services}
}

// HealthCheck returns service health status
func (h *APIHandlers) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "device-management-api",
	})
}

// --- Device Management Endpoints ---

// RegisterDevice handles new device registration
func (h *APIHandlers) RegisterDevice(c *gin.Context) {
	var device core.Device
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format", "details": err.Error()})
		return
	}

	if err := h.services.DeviceManagement.RegisterDevice(c.Request.Context(), &device); err != nil {
		switch err {
		case core.ErrDeviceAlreadyExists:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case core.ErrOrganizationNotFound, core.ErrOrganizationInactive:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register device"})
		}
		return
	}

	c.JSON(http.StatusCreated, device)
}

// GetDevice retrieves device details
func (h *APIHandlers) GetDevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}

	device, err := h.services.DeviceManagement.GetDevice(c.Request.Context(), uint(id))
	if err != nil {
		if err == core.ErrDeviceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get device"})
		}
		return
	}

	c.JSON(http.StatusOK, device)
}

// ListDevices returns devices for an organization
func (h *APIHandlers) ListDevices(c *gin.Context) {
	orgID, _ := strconv.ParseUint(c.Query("organization_id"), 10, 32)
	if orgID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization_id is required"})
		return
	}

	devices, err := h.services.DeviceManagement.ListOrganizationDevices(c.Request.Context(), uint(orgID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list devices"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"count":   len(devices),
	})
}

// UpdateDeviceStatus changes device active status
func (h *APIHandlers) UpdateDeviceStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}

	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
		return
	}

	if err := h.services.DeviceManagement.UpdateDeviceStatus(c.Request.Context(), uint(id), req.Active); err != nil {
		if err == core.ErrDeviceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update device"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "device status updated"})
}

// --- Telemetry Endpoints ---

// IngestTelemetry receives device telemetry data
func (h *APIHandlers) IngestTelemetry(c *gin.Context) {
	var telemetry core.Telemetry
	if err := c.ShouldBindJSON(&telemetry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid telemetry format"})
		return
	}

	// Get device from context (set by middleware)
	device, exists := c.Get("device")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "device authentication required"})
		return
	}

	deviceUID := device.(*core.Device).DeviceUID

	// Record heartbeat
	h.services.DeviceManagement.RecordHeartbeat(c.Request.Context(), deviceUID)

	if err := h.services.Telemetry.IngestTelemetry(c.Request.Context(), deviceUID, &telemetry); err != nil {
		switch err {
		case core.ErrDeviceNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case core.ErrDeviceInactive:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process telemetry"})
		}
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message_id": telemetry.MessageID,
		"status":     "accepted",
	})
}

// IngestBatchTelemetry receives multiple telemetry messages
func (h *APIHandlers) IngestBatchTelemetry(c *gin.Context) {
	var req struct {
		Messages []*core.Telemetry `json:"messages"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid batch format"})
		return
	}

	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty batch"})
		return
	}

	if err := h.services.Telemetry.IngestBatch(c.Request.Context(), req.Messages); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process batch"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"count":  len(req.Messages),
		"status": "accepted",
	})
}

// GetDeviceTelemetry retrieves historical telemetry
func (h *APIHandlers) GetDeviceTelemetry(c *gin.Context) {
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	telemetry, err := h.services.Telemetry.GetDeviceTelemetry(c.Request.Context(), uint(deviceID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get telemetry"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"telemetry": telemetry,
		"count":     len(telemetry),
	})
}

// --- Organization Endpoints ---

// CreateOrganization creates a new organization
func (h *APIHandlers) CreateOrganization(c *gin.Context) {
	var org core.Organization
	if err := c.ShouldBindJSON(&org); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization format"})
		return
	}

	if err := h.services.Organization.CreateOrganization(c.Request.Context(), &org); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create organization"})
		return
	}

	c.JSON(http.StatusCreated, org)
}

// ListOrganizations returns all organizations
func (h *APIHandlers) ListOrganizations(c *gin.Context) {
	orgs, err := h.services.Organization.ListOrganizations(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list organizations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"organizations": orgs,
		"count":         len(orgs),
	})
}

// --- Firmware Management Endpoints ---

// UploadFirmware handles firmware file upload
func (h *APIHandlers) UploadFirmware(c *gin.Context) {
	file, header, err := c.Request.FormFile("firmware")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "firmware file required"})
		return
	}
	defer file.Close()

	// Read file data
	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	// Get metadata from form
	metadata := core.FirmwareMetadata{
		Filename:       header.Filename,
		Version:        c.PostForm("version"),
		ReleaseChannel: c.DefaultPostForm("channel", core.ReleaseChannelAlpha),
		ReleaseNotes:   c.PostForm("release_notes"),
	}

	if metadata.Version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version is required"})
		return
	}

	firmware, err := h.services.FirmwareManagement.CreateRelease(c.Request.Context(), data, metadata)
	if err != nil {
		if businessErr, ok := err.(core.BusinessError); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": businessErr.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create firmware release"})
		}
		return
	}

	c.JSON(http.StatusCreated, firmware)
}

// ListFirmwareReleases returns available firmware
func (h *APIHandlers) ListFirmwareReleases(c *gin.Context) {
	channel := c.Query("channel")

	releases, err := h.services.FirmwareManagement.ListReleases(c.Request.Context(), channel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list releases"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"releases": releases,
		"count":    len(releases),
	})
}

// PromoteFirmwareRelease changes release status
func (h *APIHandlers) PromoteFirmwareRelease(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid firmware id"})
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
		return
	}

	validStatuses := []string{
		core.ReleaseStatusDraft,
		core.ReleaseStatusTesting,
		core.ReleaseStatusApproved,
		core.ReleaseStatusDeprecated,
	}

	valid := false
	for _, s := range validStatuses {
		if req.Status == s {
			valid = true
			break
		}
	}

	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	if err := h.services.FirmwareManagement.PromoteRelease(c.Request.Context(), uint(id), req.Status); err != nil {
		if businessErr, ok := err.(core.BusinessError); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": businessErr.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to promote release"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "release status updated"})
}

// --- OTA Update Endpoints ---

// CheckForUpdates checks if device has available updates
func (h *APIHandlers) CheckForUpdates(c *gin.Context) {
	deviceUID := c.Param("uid")
	currentVersion := c.Query("version")

	if currentVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current version required"})
		return
	}

	session, err := h.services.UpdateManagement.CheckForUpdates(c.Request.Context(), deviceUID, currentVersion)
	if err != nil {
		if err == core.ErrDeviceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check updates"})
		}
		return
	}

	if session == nil {
		c.JSON(http.StatusOK, gin.H{
			"update_available": false,
			"current_version":  currentVersion,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"update_available": true,
		"update_session":   session,
	})
}

// DownloadFirmwareChunk handles chunked firmware download
func (h *APIHandlers) DownloadFirmwareChunk(c *gin.Context) {
	sessionID := c.Param("session")
	offset, _ := strconv.ParseInt(c.Query("offset"), 10, 64)
	size, _ := strconv.ParseInt(c.DefaultQuery("size", "32768"), 10, 64)

	if size <= 0 || size > 1048576 { // Max 1MB chunks
		size = 32768 // Default 32KB
	}

	chunk, err := h.services.UpdateManagement.DownloadFirmwareChunk(c.Request.Context(), sessionID, offset, size)
	if err != nil {
		if businessErr, ok := err.(core.BusinessError); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": businessErr.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get chunk"})
		}
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.Itoa(len(chunk)))
	c.Header("X-Chunk-Offset", strconv.FormatInt(offset, 10))
	c.Header("X-Chunk-Size", strconv.Itoa(len(chunk)))

	c.Data(http.StatusOK, "application/octet-stream", chunk)
}

// CompleteUpdate marks update as complete
func (h *APIHandlers) CompleteUpdate(c *gin.Context) {
	sessionID := c.Param("session")

	var req struct {
		Checksum string `json:"checksum"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "checksum required"})
		return
	}

	if err := h.services.UpdateManagement.CompleteUpdate(c.Request.Context(), sessionID, req.Checksum); err != nil {
		switch err {
		case core.ErrChecksumMismatch:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to complete update"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "update completed successfully"})
}

// --- Admin Endpoints ---

// GetSystemStats returns system statistics
func (h *APIHandlers) GetSystemStats(c *gin.Context) {
	stats := gin.H{
		"telemetry": h.services.Telemetry.GetIngestionStats(),
		"updates":   h.services.UpdateManagement.GetUpdateMetrics(),
		"timestamp": time.Now(),
	}

	c.JSON(http.StatusOK, stats)
}
