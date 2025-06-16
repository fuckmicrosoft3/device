// api/routes/routes.go
package routes

import (
	"example.com/backstage/services/device/api/handlers"
	"example.com/backstage/services/device/api/middleware"
	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/service"
	"example.com/backstage/services/device/internal/repository"
	
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SetupRoutes sets up all the routes for the server
func SetupRoutes(r *gin.Engine, svc service.Service, repo repository.Repository, log *logrus.Logger) {
	// Health check (public)
	r.GET("/health", handlers.HealthCheck)
	
	// API routes
	api := r.Group("/api/v1")
	
	// Device routes
	deviceHandler := handlers.NewDeviceHandler(svc, log)
	devices := api.Group("/devices")
	
	// Viewer level endpoints (GET)
	devices.GET("", middleware.APIKeyAuth(repo, log, models.ViewerAuthLevel), deviceHandler.ListDevices)
	devices.GET("/:id", middleware.APIKeyAuth(repo, log, models.ViewerAuthLevel), deviceHandler.GetDevice)
	devices.GET("/:id/messages", middleware.APIKeyAuth(repo, log, models.ViewerAuthLevel), deviceHandler.GetDeviceMessages)
	
	// Writer level endpoints (POST, PATCH)
	devices.POST("", middleware.APIKeyAuth(repo, log, models.WriterAuthLevel), deviceHandler.RegisterDevice)
	devices.PATCH("/:id/status", middleware.APIKeyAuth(repo, log, models.WriterAuthLevel), deviceHandler.UpdateDeviceStatus)
	devices.POST("/:id/firmware", middleware.APIKeyAuth(repo, log, models.WriterAuthLevel), deviceHandler.AssignFirmware)
	
	// Device message endpoints (registered device level)
	devices.POST("/messages", middleware.APIKeyAuth(repo, log, models.RegisteredDeviceAuthLevel), deviceHandler.ReceiveMessage)
	devices.POST("/messages/batch", middleware.APIKeyAuth(repo, log, models.RegisteredDeviceAuthLevel), deviceHandler.ReceiveBatchMessages)
	
	// System monitoring endpoints (sudo level)
	devices.GET("/stats/processor", middleware.APIKeyAuth(repo, log, models.SudoAuthLevel), deviceHandler.GetProcessorStats)
	
	// Organization routes
	orgHandler := handlers.NewOrganizationHandler(svc, log)
	orgs := api.Group("/organizations")
	
	// Viewer level endpoints (GET)
	orgs.GET("", middleware.APIKeyAuth(repo, log, models.ViewerAuthLevel), orgHandler.ListOrganizations)
	orgs.GET("/:id", middleware.APIKeyAuth(repo, log, models.ViewerAuthLevel), orgHandler.GetOrganization)
	
	// Writer level endpoints (POST, PUT)
	orgs.POST("", middleware.APIKeyAuth(repo, log, models.WriterAuthLevel), orgHandler.CreateOrganization)
	orgs.PUT("/:id", middleware.APIKeyAuth(repo, log, models.WriterAuthLevel), orgHandler.UpdateOrganization)
	
	// Firmware routes
	firmwareHandler := handlers.NewFirmwareHandler(svc, log)
	firmware := api.Group("/firmware")
	
	// Viewer level endpoints (GET)
	firmware.GET("/releases", middleware.APIKeyAuth(repo, log, models.ViewerAuthLevel), firmwareHandler.ListFirmwareReleases)
	firmware.GET("/releases/:id", middleware.APIKeyAuth(repo, log, models.ViewerAuthLevel), firmwareHandler.GetFirmwareRelease)
	
	// Writer level endpoints (POST)
	firmware.POST("/releases", middleware.APIKeyAuth(repo, log, models.WriterAuthLevel), firmwareHandler.CreateFirmwareRelease)
	
	// Sudo level endpoints (administrative actions)
	firmware.POST("/releases/:id/activate", middleware.APIKeyAuth(repo, log, models.SudoAuthLevel), firmwareHandler.ActivateFirmwareRelease)
	
	// Direct device API routes with device authentication
	deviceAPI := api.Group("/device/:deviceUID")
	deviceAPI.Use(middleware.APIKeyAuth(repo, log, models.RegisteredDeviceAuthLevel))
	deviceAPI.Use(middleware.DeviceAuth(repo, log))
	
	// Set up device-specific routes
	deviceAPI.POST("/message", deviceHandler.ReceiveDeviceMessage)
}
