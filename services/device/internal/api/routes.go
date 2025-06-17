package api

import (
	"example.com/backstage/services/device/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SetupRoutes configures all API routes
func SetupRoutes(router *gin.Engine, handlers *APIHandlers, services *core.ServiceRegistry, logger *logrus.Logger) {
	// Global middleware
	router.Use(RequestLogger(logger))
	router.Use(ErrorHandler())
	router.Use(CORS())

	// Health check (public)
	router.GET("/health", handlers.HealthCheck)

	// API v1
	v1 := router.Group("/api/v1")

	// Public device endpoints (device authentication)
	deviceAPI := v1.Group("/device/:uid")
	deviceAPI.Use(DeviceAuthentication(services.DeviceManagement))
	{
		deviceAPI.POST("/telemetry", handlers.IngestTelemetry)
		deviceAPI.GET("/updates/check", handlers.CheckForUpdates)
		deviceAPI.GET("/updates/:session/download", handlers.DownloadFirmwareChunk)
		deviceAPI.POST("/updates/:session/complete", handlers.CompleteUpdate)
	}

	// Authenticated API endpoints
	authAPI := v1.Group("")
	authAPI.Use(TokenAuthentication(services.Authentication))
	{
		// Device management
		devices := authAPI.Group("/devices")
		devices.Use(RequireScope(services.Authentication, "devices:read"))
		{
			devices.GET("", handlers.ListDevices)
			devices.GET("/:id", handlers.GetDevice)
			devices.GET("/:id/telemetry", handlers.GetDeviceTelemetry)

			devices.POST("", RequireScope(services.Authentication, "devices:write"), handlers.RegisterDevice)
			devices.PATCH("/:id/status", RequireScope(services.Authentication, "devices:write"), handlers.UpdateDeviceStatus)
		}

		// Telemetry ingestion
		telemetry := authAPI.Group("/telemetry")
		telemetry.Use(RequireScope(services.Authentication, "telemetry:write"))
		{
			telemetry.POST("/batch", handlers.IngestBatchTelemetry)
		}

		// Organization management
		orgs := authAPI.Group("/organizations")
		orgs.Use(RequireScope(services.Authentication, "organizations:read"))
		{
			orgs.GET("", handlers.ListOrganizations)
			orgs.POST("", RequireScope(services.Authentication, "organizations:write"), handlers.CreateOrganization)
		}

		// Firmware management
		firmware := authAPI.Group("/firmware")
		firmware.Use(RequireScope(services.Authentication, "firmware:read"))
		{
			firmware.GET("/releases", handlers.ListFirmwareReleases)
			firmware.POST("/upload", RequireScope(services.Authentication, "firmware:write"), handlers.UploadFirmware)
			firmware.POST("/releases/:id/promote", RequireScope(services.Authentication, "firmware:admin"), handlers.PromoteFirmwareRelease)
		}

		// Admin endpoints
		admin := authAPI.Group("/admin")
		admin.Use(RequireScope(services.Authentication, "admin"))
		{
			admin.GET("/stats", handlers.GetSystemStats)
		}
	}
}
