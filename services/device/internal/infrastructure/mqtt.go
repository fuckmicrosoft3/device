// services/device/internal/infrastructure/mqtt.go
package infrastructure

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

// MessageHandler processes MQTT messages
type MessageHandler func(ctx context.Context, topic string, payload []byte) error

// MQTTConfig holds MQTT connection settings
type MQTTConfig struct {
	BrokerURL         string
	ClientID          string
	Username          string
	Password          string
	QoS               byte
	CleanSession      bool
	Topics            []string
	KeepAlive         time.Duration
	ConnectTimeout    time.Duration
	MaxReconnectDelay time.Duration
	TLSConfig         *tls.Config
}

// MQTTSubscriber handles MQTT connections and message processing
type MQTTSubscriber struct {
	config    MQTTConfig
	client    mqtt.Client
	logger    *logrus.Logger
	handlers  map[string]MessageHandler
	mu        sync.RWMutex
	connected bool
	shutdown  chan struct{}
	wg        sync.WaitGroup
}

// NewMQTTSubscriber creates a new MQTT subscriber
func NewMQTTSubscriber(config MQTTConfig, logger *logrus.Logger) (*MQTTSubscriber, error) {
	if config.BrokerURL == "" {
		return nil, fmt.Errorf("MQTT broker URL is required")
	}

	if config.ClientID == "" {
		config.ClientID = fmt.Sprintf("device-service-%d", time.Now().UnixNano())
	}

	return &MQTTSubscriber{
		config:   config,
		logger:   logger,
		handlers: make(map[string]MessageHandler),
		shutdown: make(chan struct{}),
	}, nil
}

// RegisterHandler registers a handler for a specific message type
func (s *MQTTSubscriber) RegisterHandler(messageType string, handler MessageHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[messageType] = handler
}

// Start connects to MQTT broker and subscribes to topics
func (s *MQTTSubscriber) Start() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(s.config.BrokerURL)
	opts.SetClientID(s.config.ClientID)

	if s.config.Username != "" {
		opts.SetUsername(s.config.Username)
	}
	if s.config.Password != "" {
		opts.SetPassword(s.config.Password)
	}

	opts.SetCleanSession(s.config.CleanSession)
	opts.SetKeepAlive(s.config.KeepAlive)
	opts.SetConnectTimeout(s.config.ConnectTimeout)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(s.config.MaxReconnectDelay)

	if s.config.TLSConfig != nil {
		opts.SetTLSConfig(s.config.TLSConfig)
	}

	// Connection handlers
	opts.SetOnConnectHandler(s.onConnect)
	opts.SetConnectionLostHandler(s.onConnectionLost)
	opts.SetReconnectingHandler(s.onReconnecting)

	// Message handler
	opts.SetDefaultPublishHandler(s.messageHandler)

	s.client = mqtt.NewClient(opts)

	// Connect to broker
	if token := s.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	s.logger.Info("MQTT subscriber started")
	return nil
}

// Stop gracefully shuts down the MQTT subscriber
func (s *MQTTSubscriber) Stop() {
	s.logger.Info("Stopping MQTT subscriber...")

	close(s.shutdown)

	// Unsubscribe from all topics
	if s.client != nil && s.client.IsConnected() {
		for _, topic := range s.config.Topics {
			if token := s.client.Unsubscribe(topic); token.Wait() && token.Error() != nil {
				s.logger.WithError(token.Error()).WithField("topic", topic).
					Error("Failed to unsubscribe from topic")
			}
		}

		// Disconnect from broker
		s.client.Disconnect(250)
	}

	s.wg.Wait()
	s.logger.Info("MQTT subscriber stopped")
}

// IsConnected returns the connection status
func (s *MQTTSubscriber) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// onConnect handles successful connection
func (s *MQTTSubscriber) onConnect(client mqtt.Client) {
	s.mu.Lock()
	s.connected = true
	s.mu.Unlock()

	s.logger.Info("Connected to MQTT broker")

	// Subscribe to all configured topics
	for _, topic := range s.config.Topics {
		if token := client.Subscribe(topic, s.config.QoS, nil); token.Wait() && token.Error() != nil {
			s.logger.WithError(token.Error()).WithField("topic", topic).
				Error("Failed to subscribe to topic")
		} else {
			s.logger.WithField("topic", topic).Info("Subscribed to topic")
		}
	}
}

// onConnectionLost handles connection loss
func (s *MQTTSubscriber) onConnectionLost(client mqtt.Client, err error) {
	s.mu.Lock()
	s.connected = false
	s.mu.Unlock()

	s.logger.WithError(err).Warn("Lost connection to MQTT broker")
}

// onReconnecting handles reconnection attempts
func (s *MQTTSubscriber) onReconnecting(client mqtt.Client, opts *mqtt.ClientOptions) {
	s.logger.Info("Attempting to reconnect to MQTT broker...")
}

// messageHandler processes incoming MQTT messages
func (s *MQTTSubscriber) messageHandler(client mqtt.Client, msg mqtt.Message) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.processMessage(msg)
	}()
}

// processMessage handles individual message processing
func (s *MQTTSubscriber) processMessage(msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	s.logger.WithFields(logrus.Fields{
		"topic":      topic,
		"message_id": msg.MessageID(),
		"qos":        msg.Qos(),
		"retained":   msg.Retained(),
		"size":       len(payload),
	}).Debug("Received MQTT message")

	// Determine message type based on topic
	messageType := s.getMessageType(topic)

	s.mu.RLock()
	handler, exists := s.handlers[messageType]
	s.mu.RUnlock()

	if !exists {
		s.logger.WithFields(logrus.Fields{
			"topic":        topic,
			"message_type": messageType,
		}).Warn("No handler registered for message type")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := handler(ctx, topic, payload); err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"topic":      topic,
			"message_id": msg.MessageID(),
		}).Error("Failed to process MQTT message")

		// Could implement retry logic or dead letter queue here
		s.handleFailedMessage(topic, payload, err)
	}
}

// getMessageType extracts message type from topic
func (s *MQTTSubscriber) getMessageType(topic string) string {
	// Example topic structure: devices/{device_id}/telemetry
	// This is a simplified example - adjust based on your topic structure
	if contains(topic, "telemetry") {
		return "telemetry"
	}
	if contains(topic, "status") {
		return "status"
	}
	if contains(topic, "command") {
		return "command"
	}
	return "unknown"
}

// handleFailedMessage handles messages that failed processing
func (s *MQTTSubscriber) handleFailedMessage(topic string, payload []byte, err error) {
	// Implement dead letter queue or retry mechanism
	// For now, just log the failure
	s.logger.WithError(err).WithFields(logrus.Fields{
		"topic":        topic,
		"payload_size": len(payload),
	}).Error("Message processing failed - would go to dead letter queue")
}

// PublishResponse publishes a response message
func (s *MQTTSubscriber) PublishResponse(topic string, payload []byte, qos byte) error {
	if !s.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	token := s.client.Publish(topic, qos, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish message: %w", token.Error())
	}

	return nil
}

// Helper function to check if string contains substring
func contains(str, substr string) bool {
	return len(str) >= len(substr) && str[len(str)-len(substr):] == substr
}
