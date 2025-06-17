// services/device/config/config.go
package config

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config holds the complete configuration for the service.
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	ServiceBus ServiceBusConfig `mapstructure:"service_bus"`
	MQTT       *MQTTConfig      `mapstructure:"mqtt"`
	Firmware   FirmwareConfig   `mapstructure:"firmware"`
	OTA        OTAConfig        `mapstructure:"ota"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Logger     *logrus.Logger
}

// ServerConfig holds the HTTP server settings.
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// DatabaseConfig holds the PostgreSQL connection settings.
type DatabaseConfig struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	EnableTracing   bool          `mapstructure:"enable_tracing"`
}

// RedisConfig holds the Redis connection settings.
type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
}

// ServiceBusConfig holds the Azure Service Bus settings.
type ServiceBusConfig struct {
	ConnectionString string        `mapstructure:"connection_string"`
	QueueName        string        `mapstructure:"queue_name"`
	MaxRetries       int           `mapstructure:"max_retries"`
	RetryDelay       time.Duration `mapstructure:"retry_delay"`
}

// MQTTConfig holds MQTT broker settings for telemetry ingestion
type MQTTConfig struct {
	BrokerURL         string        `mapstructure:"broker_url"`
	ClientID          string        `mapstructure:"client_id"`
	Username          string        `mapstructure:"username"`
	Password          string        `mapstructure:"password"`
	QoS               byte          `mapstructure:"qos"`
	CleanSession      bool          `mapstructure:"clean_session"`
	Topics            []string      `mapstructure:"topics"`
	KeepAlive         time.Duration `mapstructure:"keep_alive"`
	ConnectTimeout    time.Duration `mapstructure:"connect_timeout"`
	MaxReconnectDelay time.Duration `mapstructure:"max_reconnect_delay"`
}

// FirmwareConfig holds settings related to firmware management.
type FirmwareConfig struct {
	StoragePath    string `mapstructure:"storage_path"`
	MaxFileSize    int64  `mapstructure:"max_file_size"`
	SigningEnabled bool   `mapstructure:"signing_enabled"`
	PrivateKeyPath string `mapstructure:"private_key_path"`
}

// OTAConfig holds settings for Over-The-Air updates.
type OTAConfig struct {
	ChunkSize            int           `mapstructure:"chunk_size"`
	MaxConcurrentUpdates int           `mapstructure:"max_concurrent_updates"`
	DownloadTimeout      time.Duration `mapstructure:"download_timeout"`
	RetryAttempts        int           `mapstructure:"retry_attempts"`
}

// StorageConfig holds settings for persistent storage
type StorageConfig struct {
	WALPath        string `mapstructure:"wal_path"`
	DeadLetterPath string `mapstructure:"dead_letter_path"`
	BackupPath     string `mapstructure:"backup_path"`
	RetentionDays  int    `mapstructure:"retention_days"`
}

// Load reads configuration from a file and environment variables.
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	viper.SetEnvPrefix("DEVICE")
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "15s")
	viper.SetDefault("server.write_timeout", "15s")

	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.conn_max_lifetime", "5m")
	viper.SetDefault("database.conn_max_idle_time", "10m")
	viper.SetDefault("database.enable_tracing", false)

	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.min_idle_conns", 5)
	viper.SetDefault("redis.dial_timeout", "5s")

	viper.SetDefault("service_bus.max_retries", 3)
	viper.SetDefault("service_bus.retry_delay", "1s")

	viper.SetDefault("mqtt.qos", 1)
	viper.SetDefault("mqtt.clean_session", false)
	viper.SetDefault("mqtt.keep_alive", "30s")
	viper.SetDefault("mqtt.connect_timeout", "10s")
	viper.SetDefault("mqtt.max_reconnect_delay", "2m")

	viper.SetDefault("firmware.storage_path", "./firmware_files")
	viper.SetDefault("firmware.max_file_size", 10485760) // 10MB

	viper.SetDefault("ota.chunk_size", 32768) // 32KB
	viper.SetDefault("ota.max_concurrent_updates", 50)
	viper.SetDefault("ota.download_timeout", "10m")
	viper.SetDefault("ota.retry_attempts", 3)

	viper.SetDefault("storage.wal_path", "/data/wal")
	viper.SetDefault("storage.dead_letter_path", "/data/dead_letter")
	viper.SetDefault("storage.backup_path", "/data/backup")
	viper.SetDefault("storage.retention_days", 30)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if using env vars
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}
