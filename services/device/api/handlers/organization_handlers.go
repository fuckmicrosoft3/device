package handlers

import (
	"net/http"
	"strconv"
	
	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/service"
	
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// OrganizationHandler handles organization-related requests
type OrganizationHandler struct {
	service service.Service
	log     *logrus.Logger
}

// NewOrganizationHandler creates a new OrganizationHandler instance
func NewOrganizationHandler(svc service.Service, log *logrus.Logger) *OrganizationHandler {
	return &OrganizationHandler{
		service: svc,
		log:     log,
	}
}

// CreateOrganization handles organization creation
func (h *OrganizationHandler) CreateOrganization(c *gin.Context) {
	var org models.Organization
	if err := c.ShouldBindJSON(&org); err != nil {
		h.log.WithError(err).Warn("Invalid organization format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid organization format",
		})
		return
	}
	
	if org.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Organization name is required",
		})
		return
	}
	
	if err := h.service.CreateOrganization(c, &org); err != nil {
		h.log.WithError(err).Error("Failed to create organization")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create organization",
		})
		return
	}
	
	c.JSON(http.StatusCreated, org)
}

// GetOrganization handles organization retrieval
func (h *OrganizationHandler) GetOrganization(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid organization ID",
		})
		return
	}
	
	org, err := h.service.GetOrganization(c, uint(id))
	if err != nil {
		h.log.WithError(err).Error("Failed to get organization")
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Organization not found",
		})
		return
	}
	
	c.JSON(http.StatusOK, org)
}

// ListOrganizations handles listing all organizations
func (h *OrganizationHandler) ListOrganizations(c *gin.Context) {
	orgs, err := h.service.ListOrganizations(c)
	if err != nil {
		h.log.WithError(err).Error("Failed to list organizations")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list organizations",
		})
		return
	}
	
	c.JSON(http.StatusOK, orgs)
}

// UpdateOrganization handles organization updates
func (h *OrganizationHandler) UpdateOrganization(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid organization ID",
		})
		return
	}
	
	// Get the existing organization
	org, err := h.service.GetOrganization(c, uint(id))
	if err != nil {
		h.log.WithError(err).Error("Failed to get organization")
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Organization not found",
		})
		return
	}
	
	// Bind the updates
	if err := c.ShouldBindJSON(org); err != nil {
		h.log.WithError(err).Warn("Invalid organization format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid organization format",
		})
		return
	}
	
	// Ensure ID doesn't change
	org.ID = uint(id)
	
	if err := h.service.UpdateOrganization(c, org); err != nil {
		h.log.WithError(err).Error("Failed to update organization")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update organization",
		})
		return
	}
	
	c.JSON(http.StatusOK, org)
}