package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"os"
	"io"
	"crypto/sha256"
	"encoding/hex"
	"time"
	
	"example.com/backstage/services/device/config"
	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/service"
	"example.com/backstage/services/device/internal/utils"
	
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// FirmwareUploadHandler handles firmware upload operations
type FirmwareUploadHandler struct {
	service service.Service
	log     *logrus.Logger
	config  *config.FirmwareConfig
}

// NewFirmwareUploadHandler creates a new FirmwareUploadHandler instance
func NewFirmwareUploadHandler(svc service.Service, log *logrus.Logger, cfg *config.FirmwareConfig) *FirmwareUploadHandler {
	return &FirmwareUploadHandler{
		service: svc,
		log:     log,
		config:  cfg,
	}
}

// UploadFirmware handles firmware binary upload with form data
func (h *FirmwareUploadHandler) UploadFirmware(c *gin.Context) {
	// Parse multipart form
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32 MB limit
		h.log.WithError(err).Warn("Failed to parse multipart form")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to parse form data",
		})
		return
	}
	
	// Get form values
	version := c.PostForm("version")
	releaseType := c.PostForm("type")
	notes := c.PostForm("notes")
	isTestStr := c.PostForm("is_test")
	testDeviceIDStr := c.PostForm("test_device_id")
	
	// Validate required fields
	if version == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Version is required",
		})
		return
	}
	
	// Parse semantic version
	semVer, err := utils.ParseSemanticVersion(version)
	if err != nil {
		h.log.WithError(err).Warn("Invalid semantic version format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid semantic version: %v", err),
		})
		return
	}
	
	// Get file from form
	file, header, err := c.Request.FormFile("firmware")
	if err != nil {
		h.log.WithError(err).Warn("Failed to get firmware file")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to get firmware file",
		})
		return
	}
	defer file.Close()
	
	// Create storage directory if it doesn't exist
	firmwareDir := h.config.StoragePath
	if err := os.MkdirAll(firmwareDir, 0755); err != nil {
		h.log.WithError(err).Error("Failed to create firmware directory")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create storage directory",
		})
		return
	}
	
	// Generate filename based on version and timestamp
	timestamp := time.Now().UTC().Format("20060102150405")
	filename := fmt.Sprintf("firmware_v%s_%s_%s.bin", version, releaseType, timestamp)
	filepath := filepath.Join(firmwareDir, filename)
	
	// Create destination file
	dst, err := os.Create(filepath)
	if err != nil {
		h.log.WithError(err).Error("Failed to create file")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save firmware file",
		})
		return
	}
	defer dst.Close()
	
	// Calculate hash while copying
	hasher := sha256.New()
	writer := io.MultiWriter(dst, hasher)
	
	if _, err := io.Copy(writer, file); err != nil {
		h.log.WithError(err).Error("Failed to copy file")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save firmware file",
		})
		return
	}
	
	// Get file hash
	fileHash := hex.EncodeToString(hasher.Sum(nil))
	
	// Parse test device ID if provided
	var testDeviceID *uint
	if testDeviceIDStr != "" {
		id, err := ParseUint(testDeviceIDStr)
		if err == nil {
			uintID := uint(id)
			testDeviceID = &uintID
		}
	}
	
	// Determine if this is a test release
	isTest := isTestStr == "true" || isTestStr == "1"
	
	// Create firmware release record
	release := &models.FirmwareRelease{
		FilePath:    filepath,
		ReleaseType: models.ReleaseType(releaseType),
		Version:     version,
		Size:        uint(header.Size),
		Valid:       true, // Will be validated later
		IsTest:      isTest,
		TestDeviceID: testDeviceID,
		FileHash:    fileHash,
	}
	
	// Create extended firmware release
	extendedRelease := &models.FirmwareReleaseExtended{
		FirmwareRelease:   *release,
		MajorVersion:      semVer.Major,
		MinorVersion:      semVer.Minor,
		PatchVersion:      semVer.Patch,
		PreReleaseVersion: semVer.PreRelease,
		BuildMetadata:     semVer.Build,
		ReleaseNotes:      notes,
	}
	
	// Sign the firmware if enabled
	if h.config.SigningEnabled {
		signature, err := utils.SignFirmware(filepath, h.config.PrivateKeyPath)
		if err != nil {
			h.log.WithError(err).Error("Failed to sign firmware")
			// Continue without signature, but mark as not signed
			extendedRelease.Error = fmt.Sprintf("Signing failed: %v", err)
		} else {
			extendedRelease.Signature = signature
			extendedRelease.SignatureAlgorithm = "ecdsa-secp256r1"
			now := time.Now()
			extendedRelease.SignedAt = &now
			extendedRelease.SignedBy = "system"
		}
	}
	
	// Save to database
	if err := h.service.CreateFirmwareReleaseExtended(c, extendedRelease); err != nil {
		h.log.WithError(err).Error("Failed to create firmware release")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create firmware release record",
		})
		return
	}
	
	// Start validation process asynchronously
	go func() {
		if _, err := h.service.VerifyFirmwareRelease(context.Background(), extendedRelease.ID); err != nil {
			h.log.WithError(err).Error("Failed to verify firmware")
		}
	}()
	
	c.JSON(http.StatusCreated, gin.H{
		"status":        "success",
		"id":            extendedRelease.ID,
		"version":       version,
		"file_path":     filepath,
		"size":          header.Size,
		"hash":          fileHash,
		"release_type":  releaseType,
		"is_test":       isTest,
		"test_device_id": testDeviceID,
	})
}

// ParseUint is a helper function to parse uint values from strings
func ParseUint(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	return strconv.ParseUint(s, 10, 64)
}