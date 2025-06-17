// sdk/device-client/client.go
// Example SDK for interacting with the Device Management API
package deviceclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents the Device Management API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiToken   string
}

// NewClient creates a new API client
func NewClient(baseURL, apiToken string) *Client {
	return &Client{
		baseURL:  baseURL,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Device represents an IoT device
type Device struct {
	ID              uint       `json:"id"`
	DeviceUID       string     `json:"device_uid"`
	SerialNumber    string     `json:"serial_number"`
	OrganizationID  uint       `json:"organization_id"`
	FirmwareVersion string     `json:"firmware_version"`
	Active          bool       `json:"active"`
	UpdatesEnabled  bool       `json:"updates_enabled"`
	LastHeartbeat   *time.Time `json:"last_heartbeat"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Organization represents a device organization
type Organization struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Active      bool      `json:"active"`
	DeviceLimit int       `json:"device_limit"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FirmwareRelease represents a firmware version
type FirmwareRelease struct {
	ID               uint      `json:"id"`
	Version          string    `json:"version"`
	ReleaseChannel   string    `json:"release_channel"`
	Checksum         string    `json:"checksum"`
	SizeBytes        int64     `json:"size_bytes"`
	DigitalSignature string    `json:"digital_signature,omitempty"`
	ReleaseStatus    string    `json:"release_status"`
	ReleaseNotes     string    `json:"release_notes"`
	MinimumVersion   string    `json:"minimum_version,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// HealthStatus represents the service health
type HealthStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
}

// doRequest performs an HTTP request
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	url := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("API error: %d - %v", resp.StatusCode, errResp)
	}

	return resp, nil
}

// Health checks the service health
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var health HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &health, nil
}

// --- Organization Methods ---

// CreateOrganization creates a new organization
func (c *Client) CreateOrganization(ctx context.Context, org *Organization) (*Organization, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/organizations", org)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var created Organization
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &created, nil
}

// ListOrganizations lists all organizations
func (c *Client) ListOrganizations(ctx context.Context) ([]Organization, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/organizations", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Organizations []Organization `json:"organizations"`
		Count         int            `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Organizations, nil
}

// --- Device Methods ---

// RegisterDevice registers a new device
func (c *Client) RegisterDevice(ctx context.Context, device *Device) (*Device, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/devices", device)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var registered Device
	if err := json.NewDecoder(resp.Body).Decode(&registered); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &registered, nil
}

// GetDevice retrieves a device by ID
func (c *Client) GetDevice(ctx context.Context, id uint) (*Device, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/devices/%d", id), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var device Device
	if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &device, nil
}

// ListDevices lists devices for an organization
func (c *Client) ListDevices(ctx context.Context, organizationID uint) ([]Device, error) {
	resp, err := c.doRequest(ctx, http.MethodGet,
		fmt.Sprintf("/api/v1/devices?organization_id=%d", organizationID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Devices []Device `json:"devices"`
		Count   int      `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Devices, nil
}

// UpdateDeviceStatus updates device active status
func (c *Client) UpdateDeviceStatus(ctx context.Context, id uint, active bool) error {
	body := map[string]bool{"active": active}
	resp, err := c.doRequest(ctx, http.MethodPatch,
		fmt.Sprintf("/api/v1/devices/%d/status", id), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// --- Firmware Methods ---

// ListFirmwareReleases lists firmware releases
func (c *Client) ListFirmwareReleases(ctx context.Context, channel string) ([]FirmwareRelease, error) {
	path := "/api/v1/firmware/releases"
	if channel != "" {
		path += "?channel=" + channel
	}

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Releases []FirmwareRelease `json:"releases"`
		Count    int               `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Releases, nil
}

// PromoteFirmwareRelease changes firmware release status
func (c *Client) PromoteFirmwareRelease(ctx context.Context, id uint, status string) error {
	body := map[string]string{"status": status}
	resp, err := c.doRequest(ctx, http.MethodPost,
		fmt.Sprintf("/api/v1/firmware/releases/%d/promote", id), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// --- Admin Methods ---

// GetSystemStats retrieves system statistics
func (c *Client) GetSystemStats(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/admin/stats", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return stats, nil
}

// --- Example Usage ---
/*
func main() {
	// Create client
	client := deviceclient.NewClient("http://localhost:8080", "your-api-token")

	ctx := context.Background()

	// Check health
	health, err := client.Health(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Service status: %s\n", health.Status)

	// Create organization
	org, err := client.CreateOrganization(ctx, &deviceclient.Organization{
		Name:        "ACME Corp",
		DeviceLimit: 1000,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created organization: %s (ID: %d)\n", org.Name, org.ID)

	// Register device
	device, err := client.RegisterDevice(ctx, &deviceclient.Device{
		DeviceUID:       "device-001",
		SerialNumber:    "SN123456",
		OrganizationID:  org.ID,
		FirmwareVersion: "1.0.0",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Registered device: %s (ID: %d)\n", device.DeviceUID, device.ID)
}
*/
