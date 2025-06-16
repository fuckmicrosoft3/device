// api/handlers/device_handler.go
package handlers

import (
	"net/http"
	"strconv"
	
	"example.com/backstage/services/device/api/middleware" // Add this import
	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/service"
	
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// DeviceHandler handles device-related requests
type DeviceHandler struct {
	service service.Service
	log     *logrus.Logger
}

// NewDeviceHandler creates a new DeviceHandler instance
func NewDeviceHandler(svc service.Service, log *logrus.Logger) *DeviceHandler {
	return &DeviceHandler{
		service: svc,
		log:     log,
	}
}

// RegisterDevice handles device registration
func (h *DeviceHandler) RegisterDevice(c *gin.Context) {
	var device models.Device
	if err := c.ShouldBindJSON(&device); err != nil {
		h.log.WithError(err).Warn("Invalid device format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid device format",
		})
		return
	}
	
	// Process the device registration
	if err := h.service.RegisterDevice(c, &device); err != nil {
		h.log.WithError(err).Error("Failed to register device")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to register device",
		})
		return
	}
	
	c.JSON(http.StatusOK, device)
}

// GetDevice handles device information retrieval
func (h *DeviceHandler) GetDevice(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		// Try to get by UID if it's not a numeric ID
		device, err := h.service.GetDeviceByUID(c, idStr)
		if err != nil {
			h.log.WithError(err).Error("Failed to get device")
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Device not found",
			})
			return
		}
		c.JSON(http.StatusOK, device)
		return
	}
	
	device, err := h.service.GetDevice(c, uint(id))
	if err != nil {
		h.log.WithError(err).Error("Failed to get device")
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Device not found",
		})
		return
	}
	
	c.JSON(http.StatusOK, device)
}

// ListDevices handles listing all devices
func (h *DeviceHandler) ListDevices(c *gin.Context) {
	orgIDStr := c.Query("org_id")
	var orgID uint
	
	if orgIDStr != "" {
		id, err := strconv.ParseUint(orgIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid organization ID",
			})
			return
		}
		orgID = uint(id)
	}
	
	devices, err := h.service.ListDevices(c, orgID)
	if err != nil {
		h.log.WithError(err).Error("Failed to list devices")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list devices",
		})
		return
	}
	
	c.JSON(http.StatusOK, devices)
}

// UpdateDeviceStatus handles updating a device's status
func (h *DeviceHandler) UpdateDeviceStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid device ID",
		})
		return
	}
	
	var request struct {
		Active bool `json:"active"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}
	
	if err := h.service.UpdateDeviceStatus(c, uint(id), request.Active); err != nil {
		h.log.WithError(err).Error("Failed to update device status")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update device status",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"id":     id,
		"active": request.Active,
	})
}

// AssignFirmware handles assigning a firmware release to a device
func (h *DeviceHandler) AssignFirmware(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid device ID",
		})
		return
	}
	
	var request struct {
		ReleaseID uint `json:"release_id"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}
	
	if err := h.service.AssignFirmwareToDevice(c, uint(id), request.ReleaseID); err != nil {
		h.log.WithError(err).Error("Failed to assign firmware")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to assign firmware",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"device_id":  id,
		"release_id": request.ReleaseID,
	})
}

// ReceiveMessage handles receiving messages from devices
func (h *DeviceHandler) ReceiveMessage(c *gin.Context) {
	var message models.DeviceMessage
	if err := c.ShouldBindJSON(&message); err != nil {
		h.log.WithError(err).Warn("Invalid message format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid message format",
		})
		return
	}
	
	// Set the sent via method
	message.SentVia = "api"
	
	// Process the message
	if err := h.service.ProcessDeviceMessage(c, &message); err != nil {
		h.log.WithError(err).Error("Failed to process message")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process message",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"uuid":   message.UUID,
	})
}

// BatchMessagesRequest represents a batch of messages to process
type BatchMessagesRequest struct {
	Messages []*models.DeviceMessage `json:"messages" binding:"required,min=1"`
}

// ReceiveBatchMessages handles receiving multiple messages from devices in a single request
func (h *DeviceHandler) ReceiveBatchMessages(c *gin.Context) {
	var batchRequest BatchMessagesRequest
	if err := c.ShouldBindJSON(&batchRequest); err != nil {
		h.log.WithError(err).Warn("Invalid batch message format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid batch message format",
		})
		return
	}
	
	// Check if we have messages
	if len(batchRequest.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No messages provided in batch",
		})
		return
	}
	
	h.log.Infof("Received batch of %d messages", len(batchRequest.Messages))
	
	// Set sent via method for all messages
	for _, msg := range batchRequest.Messages {
		msg.SentVia = "api-batch"
	}
	
	// Process the batch of messages
	if err := h.service.BatchProcessMessages(c, batchRequest.Messages); err != nil {
		h.log.WithError(err).Error("Failed to process batch messages")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process batch messages",
		})
		return
	}
	
	// Return success response with processor stats
	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"count":       len(batchRequest.Messages),
		"queue_stats": h.service.GetProcessorStats(),
	})
}

// GetProcessorStats returns statistics about the message processor
func (h *DeviceHandler) GetProcessorStats(c *gin.Context) {
	stats := h.service.GetProcessorStats()
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"stats":  stats,
	})
}

// GetDeviceMessages handles retrieving device messages
func (h *DeviceHandler) GetDeviceMessages(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid device ID",
		})
		return
	}
	
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}
	
	messages, err := h.service.GetDeviceMessages(c, uint(id), limit)
	if err != nil {
		h.log.WithError(err).Error("Failed to get device messages")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get device messages",
		})
		return
	}
	
	c.JSON(http.StatusOK, messages)
}

// ReceiveDeviceMessage handles receiving a message from a specific device
func (h *DeviceHandler) ReceiveDeviceMessage(c *gin.Context) {
	// Get device from context (placed there by middleware)
	device, err := middleware.GetDeviceFromContext(c)
	if err != nil {
		h.log.WithError(err).Error("Failed to get device from context")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
		return
	}
	
	var message models.DeviceMessage
	if err := c.ShouldBindJSON(&message); err != nil {
		h.log.WithError(err).Warn("Invalid message format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid message format",
		})
		return
	}
	
	// Set the device information
	message.DeviceMCU = device.UID
	message.DeviceID = device.ID
	message.Device = device
	message.SentVia = "device-api"
	
	// Process the message
	if err := h.service.ProcessDeviceMessage(c, &message); err != nil {
		h.log.WithError(err).Error("Failed to process message")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process message",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"uuid":   message.UUID,
	})
}
