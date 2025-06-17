// services/device/internal/core/registry.go
package core

// ServiceRegistry holds all domain services
// Using interface{} to allow both interface and concrete types
type ServiceRegistry struct {
	DeviceManagement   interface{} // *DeviceManagementService
	Telemetry          interface{} // *TelemetryService
	FirmwareManagement interface{} // *FirmwareManagementService
	UpdateManagement   interface{} // *UpdateManagementService
	Organization       interface{} // *OrganizationService
	Authentication     interface{} // *AuthenticationService
}
