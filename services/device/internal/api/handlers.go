package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"example.com/backstage/services/device/internal/core"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// APIHandlers holds all HTTP handlers with concrete service types.
type APIHandlers struct {
	deviceManagement   *core.DeviceManagementService
	telemetry          *core.TelemetryService
	firmwareManagement *core.FirmwareManagementService
	updateManagement   *core.UpdateManagementService
	organization       *core.OrganizationService
	authentication     *core.AuthenticationService
}

// NewAPIHandlers creates a new handler instance with the service registry.
func NewAPIHandlers(services *core.ServiceRegistry) *APIHandlers {
	return &APIHandlers{
		deviceManagement:   services.DeviceManagement.(*core.DeviceManagementService),
		telemetry:          services.Telemetry.(*core.TelemetryService),
		firmwareManagement: services.FirmwareManagement.(*core.FirmwareManagementService),
		updateManagement:   services.UpdateManagement.(*core.UpdateManagementService),
		organization:       services.Organization.(*core.OrganizationService),
		authentication:     services.Authentication.(*core.AuthenticationService),
	}
}

// HealthCheck returns service health status.
func (h *APIHandlers) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "device-management-api",
	})
}

// --- Device Management Endpoints ---

// RegisterDevice handles new device registration.
func (h *APIHandlers) RegisterDevice(c *gin.Context) {
	var device core.Device
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format", "details": err.Error()})
		return
	}

	if err := h.deviceManagement.RegisterDevice(c.Request.Context(), &device); err != nil {
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

// GetDevice retrieves device details.
func (h *APIHandlers) GetDevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}

	device, err := h.deviceManagement.GetDevice(c.Request.Context(), uint(id))
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

// ListDevices returns devices for an organization.
func (h *APIHandlers) ListDevices(c *gin.Context) {
	orgID, _ := strconv.ParseUint(c.Query("organization_id"), 10, 32)
	if orgID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization_id is required"})
		return
	}

	devices, err := h.deviceManagement.ListOrganizationDevices(c.Request.Context(), uint(orgID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list devices"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"count":   len(devices),
	})
}

// UpdateDeviceStatus changes device active status.
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

	if err := h.deviceManagement.UpdateDeviceStatus(c.Request.Context(), uint(id), req.Active); err != nil {
		if err == core.ErrDeviceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update device status"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "device status updated"})
}

// --- Telemetry Endpoints ---

// IngestTelemetry receives device telemetry data via HTTP.
func (h *APIHandlers) IngestTelemetry(c *gin.Context) {
	rawPayload, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	device, exists := c.Get("device")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "device authentication required"})
		return
	}
	deviceObj := device.(*core.Device)

	h.deviceManagement.RecordHeartbeat(c.Request.Context(), deviceObj.DeviceUID)

	if err := h.telemetry.IngestTelemetry(c.Request.Context(), deviceObj, json.RawMessage(rawPayload)); err != nil {
		if businessErr, ok := err.(core.BusinessError); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": businessErr.Message, "code": businessErr.Code})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process telemetry"})
		}
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
}

// IngestBatchTelemetry receives multiple telemetry messages via HTTP.
func (h *APIHandlers) IngestBatchTelemetry(c *gin.Context) {
	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid batch format"})
		return
	}
	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty batch"})
		return
	}

	device, exists := c.Get("device")
	var deviceObj *core.Device
	if exists {
		deviceObj = device.(*core.Device)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device authentication required for batch ingestion"})
		return
	}

	if err := h.telemetry.IngestBatch(c.Request.Context(), deviceObj, req.Messages); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process batch"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"count": len(req.Messages), "status": "accepted"})
}

// GetDeviceTelemetry retrieves historical telemetry.
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

	telemetry, err := h.telemetry.GetDeviceTelemetry(c.Request.Context(), uint(deviceID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get telemetry"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"telemetry": telemetry, "count": len(telemetry)})
}

// --- Organization Endpoints ---

// CreateOrganization creates a new organization.
func (h *APIHandlers) CreateOrganization(c *gin.Context) {
	var org core.Organization
	if err := c.ShouldBindJSON(&org); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization format"})
		return
	}

	if err := h.organization.CreateOrganization(c.Request.Context(), &org); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create organization"})
		return
	}

	c.JSON(http.StatusCreated, org)
}

// GetOrganization retrieves a single organization by its ID.
func (h *APIHandlers) GetOrganization(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	org, err := h.organization.GetOrganization(c.Request.Context(), uint(id))
	if err != nil {
		if err == core.ErrOrganizationNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get organization"})
		}
		return
	}

	c.JSON(http.StatusOK, org)
}

// ListOrganizations returns all organizations.
func (h *APIHandlers) ListOrganizations(c *gin.Context) {
	orgs, err := h.organization.ListOrganizations(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list organizations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"organizations": orgs, "count": len(orgs)})
}

// UpdateOrganization updates an existing organization.
func (h *APIHandlers) UpdateOrganization(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	var org core.Organization
	if err := c.ShouldBindJSON(&org); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
		return
	}
	org.ID = uint(id) // Set ID from URL parameter

	if err := h.organization.UpdateOrganization(c.Request.Context(), &org); err != nil {
		if err == core.ErrOrganizationNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update organization"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "organization updated successfully"})
}

// --- Firmware Management Endpoints ---

// UploadFirmware handles firmware file upload.
func (h *APIHandlers) UploadFirmware(c *gin.Context) {
	file, header, err := c.Request.FormFile("firmware")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "firmware file required"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

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

	firmware, err := h.firmwareManagement.CreateRelease(c.Request.Context(), data, metadata)
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

// GetFirmwareRelease retrieves details for a single firmware release.
func (h *APIHandlers) GetFirmwareRelease(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid firmware id"})
		return
	}

	release, err := h.firmwareManagement.GetRelease(c.Request.Context(), uint(id))
	if err != nil {
		if err == core.ErrFirmwareNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get firmware release"})
		}
		return
	}

	c.JSON(http.StatusOK, release)
}

// ListFirmwareReleases returns available firmware.
func (h *APIHandlers) ListFirmwareReleases(c *gin.Context) {
	channel := c.Query("channel")
	releases, err := h.firmwareManagement.ListReleases(c.Request.Context(), channel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list releases"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"releases": releases, "count": len(releases)})
}

// PromoteFirmwareRelease changes release status.
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

	if err := h.firmwareManagement.PromoteRelease(c.Request.Context(), uint(id), req.Status); err != nil {
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

// InitiateUpdate allows an admin to trigger an update for a device.
func (h *APIHandlers) InitiateUpdate(c *gin.Context) {
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}

	var req struct {
		FirmwareID uint `json:"firmware_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request, firmware_id is required"})
		return
	}

	session, err := h.updateManagement.InitiateUpdate(c.Request.Context(), uint(deviceID), req.FirmwareID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "device or firmware not found"})
		} else if businessErr, ok := err.(core.BusinessError); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": businessErr.Message, "code": businessErr.Code})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initiate update"})
		}
		return
	}

	c.JSON(http.StatusAccepted, session)
}

// CheckForUpdates checks if a device has available updates.
func (h *APIHandlers) CheckForUpdates(c *gin.Context) {
	deviceUID := c.Param("uid")
	currentVersion := c.Query("version")
	if currentVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current version required"})
		return
	}

	session, err := h.updateManagement.CheckForUpdates(c.Request.Context(), deviceUID, currentVersion)
	if err != nil {
		if err == core.ErrDeviceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check for updates"})
		}
		return
	}

	if session == nil {
		c.JSON(http.StatusOK, gin.H{"update_available": false, "current_version": currentVersion})
		return
	}

	c.JSON(http.StatusOK, gin.H{"update_available": true, "update_session": session})
}

// GetUpdateSession retrieves details for a specific update session.
func (h *APIHandlers) GetUpdateSession(c *gin.Context) {
	sessionID := c.Param("sessionID")
	session, err := h.updateManagement.GetUpdateSession(c.Request.Context(), sessionID)
	if err != nil {
		if businessErr, ok := err.(core.BusinessError); ok {
			c.JSON(http.StatusNotFound, gin.H{"error": businessErr.Message, "code": businessErr.Code})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get update session"})
		}
		return
	}

	c.JSON(http.StatusOK, session)
}

// DownloadFirmwareChunk handles chunked firmware download.
func (h *APIHandlers) DownloadFirmwareChunk(c *gin.Context) {
	sessionID := c.Param("session")
	offset, _ := strconv.ParseInt(c.Query("offset"), 10, 64)
	size, _ := strconv.ParseInt(c.DefaultQuery("size", "32768"), 10, 64)

	chunk, err := h.updateManagement.DownloadFirmwareChunk(c.Request.Context(), sessionID, offset, size)
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
	c.Data(http.StatusOK, "application/octet-stream", chunk)
}

// CompleteUpdate marks an update as complete.
func (h *APIHandlers) CompleteUpdate(c *gin.Context) {
	sessionID := c.Param("session")
	var req struct {
		Checksum string `json:"checksum"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "checksum required"})
		return
	}

	if err := h.updateManagement.CompleteUpdate(c.Request.Context(), sessionID, req.Checksum); err != nil {
		if err == core.ErrChecksumMismatch {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to complete update"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "update completed successfully"})
}

// --- Admin & Auth Endpoints ---

// GetSystemStats returns system statistics.
func (h *APIHandlers) GetSystemStats(c *gin.Context) {
	stats := gin.H{
		"telemetry": h.telemetry.GetIngestionStats(),
		"updates":   h.updateManagement.GetUpdateMetrics(),
		"timestamp": time.Now(),
	}
	c.JSON(http.StatusOK, stats)
}

// CreateAccessToken generates a new API access token.
func (h *APIHandlers) CreateAccessToken(c *gin.Context) {
	var req struct {
		Description string   `binding:"required" json:"description"`
		Scopes      []string `binding:"required" json:"scopes"`
		ExpiresIn   string   `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format", "details": err.Error()})
		return
	}

	var duration time.Duration
	var err error
	if req.ExpiresIn != "" {
		duration, err = time.ParseDuration(req.ExpiresIn)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid duration format for expires_in"})
			return
		}
	}

	token, err := h.authentication.CreateToken(c.Request.Context(), req.Description, req.Scopes, duration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create access token"})
		return
	}

	c.JSON(http.StatusCreated, token)
}

// ListAccessTokens lists all created access tokens.
func (h *APIHandlers) ListAccessTokens(c *gin.Context) {
	tokens, err := h.authentication.ListAccessTokens(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list access tokens"})
		return
	}
	c.JSON(http.StatusOK, tokens)

	c.JSON(http.StatusNotImplemented, gin.H{"error": "this endpoint is not yet implemented"})
}

// RevokeAccessToken deletes an access token.
func (h *APIHandlers) RevokeAccessToken(c *gin.Context) {
	tokenString := c.Param("id")
	if tokenString == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token must be provided in the URL path"})
		return
	}

	if err := h.authentication.RevokeToken(c.Request.Context(), tokenString); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "token revoked successfully"})
}

// --- Batch Device Registration ---

// RegisterDeviceBatch handles batch device registration requests.
func (h *APIHandlers) RegisterDeviceBatch(c *gin.Context) {
	var requests []core.BatchRegistrationRequest
	if err := c.ShouldBindJSON(&requests); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format", "details": err.Error()})
		return
	}

	if len(requests) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no devices provided for registration"})
		return
	}

	if len(requests) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 100 devices allowed per batch"})
		return
	}

	response, err := h.deviceManagement.RegisterDeviceBatch(c.Request.Context(), requests)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "batch registration failed", "details": err.Error()})
		return
	}

	// Return 207 Multi-Status if there were partial failures
	if len(response.Failed) > 0 && len(response.Successful) > 0 {
		c.JSON(http.StatusMultiStatus, response)
		return
	}

	// Return 400 if all failed
	if len(response.Successful) == 0 {
		c.JSON(http.StatusBadRequest, response)
		return
	}

	// Return 201 if all succeeded
	c.JSON(http.StatusCreated, response)
}

// --- Enhanced OTA Handlers ---

// AcknowledgeUpdate handles device acknowledgment of update initiation.
func (h *APIHandlers) AcknowledgeUpdate(c *gin.Context) {
	sessionID := c.Param("session")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session ID is required"})
		return
	}

	if err := h.updateManagement.AcknowledgeUpdate(c.Request.Context(), sessionID); err != nil {
		if businessErr, ok := err.(core.BusinessError); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": businessErr.Message, "code": businessErr.Code})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to acknowledge update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "update acknowledged"})
}

// CompleteFlash handles device notification of firmware flash completion.
func (h *APIHandlers) CompleteFlash(c *gin.Context) {
	sessionID := c.Param("session")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session ID is required"})
		return
	}

	if err := h.updateManagement.CompleteFlash(c.Request.Context(), sessionID); err != nil {
		if businessErr, ok := err.(core.BusinessError); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": businessErr.Message, "code": businessErr.Code})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to complete flash"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "firmware flash completed"})
}

// --- Batch Update Management Handlers ---

// CreateUpdateBatch creates a new batch update job.
func (h *APIHandlers) CreateUpdateBatch(c *gin.Context) {
	var req core.CreateUpdateBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format", "details": err.Error()})
		return
	}

	if len(req.DeviceIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no devices specified for batch update"})
		return
	}

	if len(req.DeviceIDs) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 500 devices allowed per batch"})
		return
	}

	batch, err := h.updateManagement.CreateUpdateBatch(c.Request.Context(), req)
	if err != nil {
		if businessErr, ok := err.(core.BusinessError); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": businessErr.Message, "code": businessErr.Code})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create update batch", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, batch)
}

// ListUpdateBatches returns all update batches.
func (h *APIHandlers) ListUpdateBatches(c *gin.Context) {
	batches, err := h.updateManagement.ListUpdateBatches(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list update batches"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"batches": batches})
}

// GetUpdateBatch returns details of a specific update batch.
func (h *APIHandlers) GetUpdateBatch(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid batch ID"})
		return
	}

	batch, err := h.updateManagement.GetUpdateBatch(c.Request.Context(), uint(id))
	if err != nil {
		if businessErr, ok := err.(core.BusinessError); ok {
			c.JSON(http.StatusNotFound, gin.H{"error": businessErr.Message, "code": businessErr.Code})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get update batch"})
		return
	}

	c.JSON(http.StatusOK, batch)
}
