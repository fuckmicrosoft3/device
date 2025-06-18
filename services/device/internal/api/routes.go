package api

import (
	"example.com/backstage/services/device/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SetupRoutes configures all API routes, including new endpoints for full service utilization.
func SetupRoutes(router *gin.Engine, handlers *APIHandlers, services *core.ServiceRegistry, logger *logrus.Logger) {
	deviceManagement := services.DeviceManagement.(*core.DeviceManagementService)
	authService := services.Authentication.(*core.AuthenticationService)

	// --- Global Middleware ---
	router.Use(Recovery(logger))
	router.Use(RequestLogger(logger))
	router.Use(ErrorHandler())
	router.Use(CORS())

	// --- Public Endpoints ---
	router.GET("/health", handlers.HealthCheck)

	// --- API v2 Group ---
	v2 := router.Group("/api/v2")
	v2.Use(RateLimiter(100)) // Apply rate limiting to all v2 endpoints

	// --- Public Device Endpoints (Device-level Authentication) ---
	// These endpoints are called by the devices themselves.
	deviceAPI := v2.Group("/device/:uid")
	deviceAPI.Use(DeviceAuthentication(deviceManagement))
	{
		// Telemetry ingestion
		deviceAPI.POST("/telemetry", handlers.IngestTelemetry)

		// OTA update endpoints for devices
		deviceAPI.GET("/updates/check", handlers.CheckForUpdates)
		deviceAPI.GET("/updates/:session/download", handlers.DownloadFirmwareChunk)
		deviceAPI.POST("/updates/:session/complete", handlers.CompleteUpdate)
	}

	// --- Authenticated Admin/User API Endpoints (Bearer Token Authentication) ---
	// These endpoints are called by users/administrators via a UI or API client.
	authAPI := v2.Group("")
	authAPI.Use(TokenAuthentication(authService))
	{
		// --- Device Management ---
		devices := authAPI.Group("/devices")
		devices.Use(RequireScope(authService, "devices:read"))
		{
			devices.GET("", handlers.ListDevices)
			devices.GET("/:id", handlers.GetDevice)
			devices.GET("/:id/telemetry", handlers.GetDeviceTelemetry)

			// Write-scoped device endpoints
			devices.POST("", RequireScope(authService, "devices:write"), handlers.RegisterDevice)
			devices.PATCH("/:id/status", RequireScope(authService, "devices:write"), handlers.UpdateDeviceStatus)
			// [NEW] Admin endpoint to manually trigger an update check for a device
			devices.POST("/:id/update", RequireScope(authService, "devices:write"), handlers.InitiateUpdate)
		}

		// --- Telemetry Management ---
		telemetry := authAPI.Group("/telemetry")
		telemetry.Use(RequireScope(authService, "telemetry:write"))
		{
			telemetry.POST("/batch", handlers.IngestBatchTelemetry)
		}

		// --- Organization Management ---
		orgs := authAPI.Group("/organizations")
		orgs.Use(RequireScope(authService, "organizations:read"))
		{
			orgs.GET("", handlers.ListOrganizations)
			// [NEW] Get a single organization by its ID
			orgs.GET("/:id", handlers.GetOrganization)

			// Write-scoped organization endpoints
			orgs.POST("", RequireScope(authService, "organizations:write"), handlers.CreateOrganization)
			// [NEW] Update an existing organization
			orgs.PUT("/:id", RequireScope(authService, "organizations:write"), handlers.UpdateOrganization)
		}

		// --- Firmware Management ---
		firmware := authAPI.Group("/firmware")
		firmware.Use(RequireScope(authService, "firmware:read"))
		{
			firmware.GET("/releases", handlers.ListFirmwareReleases)
			// [NEW] Get a single firmware release by its ID
			firmware.GET("/releases/:id", handlers.GetFirmwareRelease)

			// Write-scoped firmware endpoints
			firmware.POST("/upload", RequireScope(authService, "firmware:write"), handlers.UploadFirmware)
			// Admin-scoped firmware endpoints
			firmware.POST("/releases/:id/promote", RequireScope(authService, "firmware:admin"), handlers.PromoteFirmwareRelease)
		}

		// --- Update Management (Admin) ---
		updates := authAPI.Group("/updates")
		updates.Use(RequireScope(authService, "admin")) // Use a general admin scope for monitoring
		{
			// [NEW] Get details of a specific update session
			updates.GET("/sessions/:sessionID", handlers.GetUpdateSession)
		}

		// --- Administrative Endpoints ---
		admin := authAPI.Group("/admin")
		admin.Use(RequireScope(authService, "admin"))
		{
			admin.GET("/stats", handlers.GetSystemStats)

			// --- Token Management ---
			tokens := admin.Group("/tokens")
			{
				// [NEW] Create a new access token
				tokens.POST("", handlers.CreateAccessToken)
				// [NEW] List all access tokens
				tokens.GET("", handlers.ListAccessTokens)
				// [NEW] Revoke an existing access token by its ID or token string
				tokens.DELETE("/:id", handlers.RevokeAccessToken)
			}
		}
	}
}
