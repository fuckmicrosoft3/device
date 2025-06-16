package http

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"example.com/backstage/services/device/internal/core"
	"github.com/gin-gonic/gin"
)

type Handlers struct {
	service core.Service
}

func NewHandlers(service core.Service) *Handlers {
	return &Handlers{service: service}
}

// Health check.
func (h *Handlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now(),
	})
}

// Device handlers

func (h *Handlers) RegisterDevice(c *gin.Context) {
	var device core.Device
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.RegisterDevice(c.Request.Context(), &device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, device)
}

func (h *Handlers) GetDevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}

	device, err := h.service.GetDevice(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	c.JSON(http.StatusOK, device)
}

func (h *Handlers) ListDevices(c *gin.Context) {
	orgID, _ := strconv.ParseUint(c.Query("org_id"), 10, 32)

	devices, err := h.service.ListDevices(c.Request.Context(), uint(orgID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, devices)
}

func (h *Handlers) UpdateDeviceStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}

	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateDeviceStatus(c.Request.Context(), uint(id), req.Active); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// Message handlers

func (h *Handlers) ReceiveMessage(c *gin.Context) {
	var message core.DeviceMessage
	if err := c.ShouldBindJSON(&message); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get device from context (set by middleware)
	device, _ := c.Get("device")
	if d, ok := device.(*core.Device); ok {
		message.Device = d
	}

	if err := h.service.ProcessMessage(c.Request.Context(), &message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"uuid": message.UUID})
}

func (h *Handlers) ReceiveBatchMessages(c *gin.Context) {
	var req struct {
		Messages []*core.DeviceMessage `json:"messages"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ProcessMessageBatch(c.Request.Context(), req.Messages); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"count": len(req.Messages)})
}

// Organization handlers

func (h *Handlers) CreateOrganization(c *gin.Context) {
	var org core.Organization
	if err := c.ShouldBindJSON(&org); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.CreateOrganization(c.Request.Context(), &org); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, org)
}

func (h *Handlers) ListOrganizations(c *gin.Context) {
	orgs, err := h.service.ListOrganizations(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, orgs)
}

// Firmware handlers

func (h *Handlers) UploadFirmware(c *gin.Context) {
	// Parse multipart form
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

	version := c.PostForm("version")
	releaseType := c.PostForm("type")

	firmware, err := h.service.UploadFirmware(c.Request.Context(), data, header.Filename, version, releaseType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, firmware)
}

func (h *Handlers) ListFirmware(c *gin.Context) {
	releaseType := c.Query("type")

	releases, err := h.service.ListFirmware(c.Request.Context(), releaseType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, releases)
}

func (h *Handlers) ActivateFirmware(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid firmware id"})
		return
	}

	if err := h.service.ActivateFirmware(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "activated"})
}

// OTA handlers

func (h *Handlers) CheckUpdate(c *gin.Context) {
	deviceUID := c.Param("uid")
	currentVersion := c.Query("version")

	session, err := h.service.CheckDeviceUpdate(c.Request.Context(), deviceUID, currentVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if session == nil {
		c.JSON(http.StatusOK, gin.H{"update_available": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"update_available": true,
		"session":          session,
	})
}

func (h *Handlers) DownloadChunk(c *gin.Context) {
	sessionID := c.Param("session")
	offset, _ := strconv.ParseInt(c.Query("offset"), 10, 64)
	size, _ := strconv.ParseInt(c.Query("size"), 10, 64)

	if size == 0 {
		size = 32768 // Default 32KB
	}

	chunk, err := h.service.ProcessOTAChunk(c.Request.Context(), sessionID, offset, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.Itoa(len(chunk)))
	c.Data(http.StatusOK, "application/octet-stream", chunk)
}

func (h *Handlers) CompleteDownload(c *gin.Context) {
	sessionID := c.Param("session")

	var req struct {
		Checksum string `json:"checksum"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.CompleteOTADownload(c.Request.Context(), sessionID, req.Checksum); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "completed"})
}

// Stats handler

func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.service.GetStats()
	c.JSON(http.StatusOK, stats)
}
