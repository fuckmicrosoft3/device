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
	Firmware   FirmwareConfig   `mapstructure:"firmware"`
	OTA        OTAConfig        `mapstructure:"ota"`
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
}

// RedisConfig holds the Redis connection settings.
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// ServiceBusConfig holds the Azure Service Bus settings.
type ServiceBusConfig struct {
	ConnectionString string `mapstructure:"connection_string"`
	QueueName        string `mapstructure:"queue_name"`
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
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("firmware.storage_path", "./firmware_files")
	viper.SetDefault("firmware.max_file_size", 10485760) // 10MB
	viper.SetDefault("ota.chunk_size", 32768)            // 32KB
	viper.SetDefault("ota.max_concurrent_updates", 50)
	viper.SetDefault("ota.download_timeout", "10m")
	viper.SetDefault("ota.retry_attempts", 3)

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
