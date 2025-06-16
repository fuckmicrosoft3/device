package http

import (
	"example.com/backstage/services/device/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func SetupRoutes(router *gin.Engine, handlers *Handlers, repo core.Repository, logger *logrus.Logger) {
	// Global middleware
	router.Use(Logger(logger))
	router.Use(ErrorHandler())
	router.Use(CORS())

	// Health check (public)
	router.GET("/health", handlers.Health)

	// API v1 routes
	v1 := router.Group("/api/v1")

	// Public device endpoints
	device := v1.Group("/device/:uid")
	device.Use(DeviceAuth(repo))
	{
		device.POST("/message", handlers.ReceiveMessage)
		device.GET("/update/check", handlers.CheckUpdate)
		device.GET("/update/:session/chunk", handlers.DownloadChunk)
		device.POST("/update/:session/complete", handlers.CompleteDownload)
	}

	// Authenticated endpoints
	auth := v1.Group("")
	auth.Use(APIKeyAuth(repo))
	{
		// Device management
		devices := auth.Group("/devices")
		devices.Use(RequirePermission("devices:read"))
		{
			devices.GET("", handlers.ListDevices)
			devices.GET("/:id", handlers.GetDevice)

			devices.POST("", RequirePermission("devices:write"), handlers.RegisterDevice)
			devices.PATCH("/:id/status", RequirePermission("devices:write"), handlers.UpdateDeviceStatus)
		}

		// Messages
		messages := auth.Group("/messages")
		messages.Use(RequirePermission("messages:write"))
		{
			messages.POST("/batch", handlers.ReceiveBatchMessages)
		}

		// Organizations
		orgs := auth.Group("/organizations")
		orgs.Use(RequirePermission("organizations:read"))
		{
			orgs.GET("", handlers.ListOrganizations)
			orgs.POST("", RequirePermission("organizations:write"), handlers.CreateOrganization)
		}

		// Firmware
		firmware := auth.Group("/firmware")
		firmware.Use(RequirePermission("firmware:read"))
		{
			firmware.GET("", handlers.ListFirmware)
			firmware.POST("/upload", RequirePermission("firmware:write"), handlers.UploadFirmware)
			firmware.POST("/:id/activate", RequirePermission("firmware:admin"), handlers.ActivateFirmware)
		}

		// Admin endpoints
		admin := auth.Group("/admin")
		admin.Use(RequirePermission("admin"))
		{
			admin.GET("/stats", handlers.GetStats)
		}
	}
}
