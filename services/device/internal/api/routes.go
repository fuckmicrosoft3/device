// services/device/internal/api/routes.go
package api

import (
	"example.com/backstage/services/device/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SetupRoutes configures all API routes with concrete service types
func SetupRoutes(router *gin.Engine, handlers *APIHandlers, services *core.ServiceRegistry, logger *logrus.Logger) {
	// Extract concrete services
	deviceManagement := services.DeviceManagement.(*core.DeviceManagementService)
	authService := services.Authentication.(*core.AuthenticationService)

	// Global middleware
	router.Use(Recovery(logger))
	router.Use(RequestLogger(logger))
	router.Use(ErrorHandler())
	router.Use(CORS())

	// Health check (public)
	router.GET("/health", handlers.HealthCheck)

	// API v1
	v1 := router.Group("/api/v1")

	// Apply rate limiting to all v1 endpoints
	v1.Use(RateLimiter(100)) // 100 requests per minute per IP

	// Public device endpoints (device authentication)
	deviceAPI := v1.Group("/device/:uid")
	deviceAPI.Use(DeviceAuthentication(deviceManagement))
	{
		// Telemetry ingestion via HTTP (also available via MQTT)
		deviceAPI.POST("/telemetry", handlers.IngestTelemetry)

		// OTA update endpoints
		deviceAPI.GET("/updates/check", handlers.CheckForUpdates)
		deviceAPI.GET("/updates/:session/download", handlers.DownloadFirmwareChunk)
		deviceAPI.POST("/updates/:session/complete", handlers.CompleteUpdate)
	}

	// Authenticated API endpoints
	authAPI := v1.Group("")
	authAPI.Use(TokenAuthentication(authService))
	{
		// Device management
		devices := authAPI.Group("/devices")
		devices.Use(RequireScope(authService, "devices:read"))
		{
			devices.GET("", handlers.ListDevices)
			devices.GET("/:id", handlers.GetDevice)
			devices.GET("/:id/telemetry", handlers.GetDeviceTelemetry)

			devices.POST("", RequireScope(authService, "devices:write"), handlers.RegisterDevice)
			devices.PATCH("/:id/status", RequireScope(authService, "devices:write"), handlers.UpdateDeviceStatus)
		}

		// Telemetry batch ingestion
		telemetry := authAPI.Group("/telemetry")
		telemetry.Use(RequireScope(authService, "telemetry:write"))
		{
			telemetry.POST("/batch", handlers.IngestBatchTelemetry)
		}

		// Organization management
		orgs := authAPI.Group("/organizations")
		orgs.Use(RequireScope(authService, "organizations:read"))
		{
			orgs.GET("", handlers.ListOrganizations)
			orgs.POST("", RequireScope(authService, "organizations:write"), handlers.CreateOrganization)
		}

		// Firmware management
		firmware := authAPI.Group("/firmware")
		firmware.Use(RequireScope(authService, "firmware:read"))
		{
			firmware.GET("/releases", handlers.ListFirmwareReleases)
			firmware.GET("/releases/:id/test-results", handlers.GetFirmwareTestResults)
			firmware.POST("/upload", RequireScope(authService, "firmware:write"), handlers.UploadFirmware)
			firmware.POST("/releases/:id/promote", RequireScope(authService, "firmware:admin"), handlers.PromoteFirmwareRelease)
		}

		// Admin endpoints
		admin := authAPI.Group("/admin")
		admin.Use(RequireScope(authService, "admin"))
		{
			admin.GET("/stats", handlers.GetSystemStats)
		}
	}

	// WebSocket endpoint for real-time telemetry (optional)
	// v1.GET("/ws/telemetry", RequireWebSocketAuth(authService), handlers.WebSocketTelemetry)
}
