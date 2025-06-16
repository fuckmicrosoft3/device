package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the service
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	MessageBus MessageBusConfig
	Redis      RedisConfig
	Logging    LoggingConfig
	API        APIConfig
}

// ServerConfig holds http server configuration
type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	CorsWhiteList   []string      `mapstructure:"cors_white_list"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host     string        `mapstructure:"host"`
	Port     int           `mapstructure:"port"`
	User     string        `mapstructure:"user"`
	Password string        `mapstructure:"password"`
	Name     string        `mapstructure:"name"`
	SSLMode  string        `mapstructure:"ssl_mode"`
	Debug    bool          `mapstructure:"debug"`
	MaxConn  int           `mapstructure:"max_conn"`
	MaxIdle  int           `mapstructure:"max_idle"`
	MaxLife  time.Duration `mapstructure:"max_life"`
}

// MessageBusConfig holds message bus configuration
type MessageBusConfig struct {
	ConnectionString string   `mapstructure:"connection_string"`
	Prefix           string   `mapstructure:"prefix"`
	Queues           []string `mapstructure:"queues"`
	ERPQueue         string   `mapstructure:"erp_queue"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	Enabled  bool   `mapstructure:"enabled"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level string `mapstructure:"level"`
	JSON  bool   `mapstructure:"json"`
}

// APIConfig holds API configuration
type APIConfig struct {
	Version string `mapstructure:"version"`
	Prefix  string `mapstructure:"prefix"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Read from config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	
	// Override with environment variables
	v.SetEnvPrefix("OPS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Create nested configuration structure
	v.SetDefault("server", v.GetStringMap("server"))
	v.SetDefault("database", v.GetStringMap("database"))
	v.SetDefault("message_bus", v.GetStringMap("message_bus"))
	v.SetDefault("redis", v.GetStringMap("redis"))
	v.SetDefault("logging", v.GetStringMap("logging"))
	v.SetDefault("api", v.GetStringMap("api"))

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

// GetConnectionString builds the database connection string
func (c *DatabaseConfig) GetConnectionString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

// GetRedisURL builds the Redis URL
func (c *RedisConfig) GetRedisURL() string {
	if c.Password == "" {
		return fmt.Sprintf("redis://%s:%d/%d", c.Host, c.Port, c.DB)
	}
	return fmt.Sprintf("redis://:%s@%s:%d/%d", c.Password, c.Host, c.Port, c.DB)
}

// setDefaults sets default values for configuration
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8000)
	v.SetDefault("server.read_timeout", "1m")
	v.SetDefault("server.write_timeout", "1m")
	v.SetDefault("server.shutdown_timeout", "10s")
	v.SetDefault("server.cors_white_list", []string{"*"})

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.name", "truck")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.debug", false)
	v.SetDefault("database.max_conn", 100)
	v.SetDefault("database.max_idle", 10)
	v.SetDefault("database.max_life", "5m")

	// MessageBus defaults
	v.SetDefault("message_bus.prefix", "dev")
	v.SetDefault("message_bus.queues", []string{"erp-operations", "truck-events"})
	v.SetDefault("message_bus.erp_queue", "erp-messages-operations")

	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 1)
	v.SetDefault("redis.enabled", true)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.json", false)

	// API defaults
	v.SetDefault("api.version", "v1")
	v.SetDefault("api.prefix", "/api")
}