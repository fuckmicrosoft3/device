package core

// ServiceRegistry holds all domain services
type ServiceRegistry struct {
	DeviceManagement   interface{} // *DeviceManagementService
	Telemetry          interface{} // *TelemetryService
	FirmwareManagement interface{} // *FirmwareManagementService
	UpdateManagement   interface{} // *UpdateManagementService
	Organization       interface{} // *OrganizationService
	Authentication     interface{} // *AuthenticationService
}
