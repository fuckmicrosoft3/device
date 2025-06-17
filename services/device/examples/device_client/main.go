// examples/device_client/main.go
// Example device client that sends telemetry via MQTT
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

// DeviceTelemetry represents the telemetry payload
type DeviceTelemetry struct {
	DeviceUID string         `json:"device_uid"`
	Telemetry *TelemetryData `json:"telemetry"`
}

// TelemetryData represents the actual telemetry data
type TelemetryData struct {
	MessageID   string                 `json:"message_id"`
	Payload     map[string]interface{} `json:"payload"`
	PublishedAt time.Time              `json:"published_at"`
}

// DeviceClient represents an IoT device
type DeviceClient struct {
	DeviceUID    string
	MQTTClient   mqtt.Client
	PublishTopic string
}

// NewDeviceClient creates a new device client
func NewDeviceClient(deviceUID, brokerURL, username, password string) (*DeviceClient, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID(fmt.Sprintf("device-%s", deviceUID))

	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}

	opts.SetCleanSession(false)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)

	// Connection event handlers
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		log.Printf("Device %s connected to MQTT broker", deviceUID)
	})

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.Printf("Device %s lost connection: %v", deviceUID, err)
	})

	client := mqtt.NewClient(opts)

	// Connect to broker
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect: %w", token.Error())
	}

	return &DeviceClient{
		DeviceUID:    deviceUID,
		MQTTClient:   client,
		PublishTopic: fmt.Sprintf("devices/%s/telemetry", deviceUID),
	}, nil
}

// SendTelemetry sends telemetry data to the MQTT broker
func (d *DeviceClient) SendTelemetry(data map[string]interface{}) error {
	telemetry := &DeviceTelemetry{
		DeviceUID: d.DeviceUID,
		Telemetry: &TelemetryData{
			MessageID:   uuid.New().String(),
			Payload:     data,
			PublishedAt: time.Now(),
		},
	}

	payload, err := json.Marshal(telemetry)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry: %w", err)
	}

	token := d.MQTTClient.Publish(d.PublishTopic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish: %w", token.Error())
	}

	log.Printf("Telemetry sent - MessageID: %s", telemetry.Telemetry.MessageID)
	return nil
}

// SendBatchTelemetry sends multiple telemetry messages
func (d *DeviceClient) SendBatchTelemetry(messages []map[string]interface{}) error {
	var batch []*TelemetryData

	for _, msg := range messages {
		batch = append(batch, &TelemetryData{
			MessageID:   uuid.New().String(),
			Payload:     msg,
			PublishedAt: time.Now(),
		})
	}

	batchPayload := map[string]interface{}{
		"device_uid": d.DeviceUID,
		"messages":   batch,
	}

	payload, err := json.Marshal(batchPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal batch: %w", err)
	}

	batchTopic := fmt.Sprintf("devices/%s/telemetry/batch", d.DeviceUID)
	token := d.MQTTClient.Publish(batchTopic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish batch: %w", token.Error())
	}

	log.Printf("Batch telemetry sent - %d messages", len(batch))
	return nil
}

// Disconnect cleanly disconnects from the MQTT broker
func (d *DeviceClient) Disconnect() {
	d.MQTTClient.Disconnect(250)
	log.Printf("Device %s disconnected", d.DeviceUID)
}

// Example usage
func main() {
	// Device configuration
	deviceUID := "device-001"
	brokerURL := "tcp://localhost:1883"
	username := ""
	password := ""

	// Create device client
	device, err := NewDeviceClient(deviceUID, brokerURL, username, password)
	if err != nil {
		log.Fatalf("Failed to create device client: %v", err)
	}
	defer device.Disconnect()

	// Send telemetry every 10 seconds
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	log.Println("Starting telemetry transmission...")

	for {
		select {
		case <-ticker.C:
			// Generate sample telemetry data
			telemetryData := map[string]interface{}{
				"temperature": 20.0 + rand.Float64()*10.0,
				"humidity":    40.0 + rand.Float64()*20.0,
				"pressure":    1000.0 + rand.Float64()*50.0,
				"battery":     rand.Float64() * 100.0,
				"timestamp":   time.Now().Unix(),
			}

			// Send telemetry
			if err := device.SendTelemetry(telemetryData); err != nil {
				log.Printf("Failed to send telemetry: %v", err)
			}

			// Occasionally send batch telemetry
			if rand.Intn(5) == 0 {
				batchData := []map[string]interface{}{
					{
						"event": "sensor_reading",
						"value": rand.Float64() * 100,
						"unit":  "percent",
					},
					{
						"event":  "status_update",
						"status": "operational",
						"uptime": time.Now().Unix(),
					},
				}

				if err := device.SendBatchTelemetry(batchData); err != nil {
					log.Printf("Failed to send batch telemetry: %v", err)
				}
			}
		}
	}
}
