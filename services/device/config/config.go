package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the service configuration
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	ServiceBus ServiceBusConfig `mapstructure:"service_bus"`
	NewRelic   NewRelicConfig   `mapstructure:"new_relic"`
	Firmware   FirmwareConfig   `mapstructure:"firmware"`
	OTA        OTAConfig        `mapstructure:"ota"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// ServerConfig holds the HTTP server configuration
type ServerConfig struct {
	Port        int           `mapstructure:"port"`
	Mode        string        `mapstructure:"mode"` // debug, release, test
	Timeout     time.Duration `mapstructure:"timeout"`
	CorsEnabled bool          `mapstructure:"cors_enabled"`
	CorsOrigins []string      `mapstructure:"cors_origins"`
}

// DatabaseConfig holds the database configuration
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// RedisConfig holds the Redis configuration
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	Enabled  bool   `mapstructure:"enabled"`
}

// ServiceBusConfig holds the Azure Service Bus configuration
type ServiceBusConfig struct {
	ConnectionString string `mapstructure:"connection_string"`
	QueueName        string `mapstructure:"queue_name"`
}

// NewRelicConfig holds the New Relic configuration
type NewRelicConfig struct {
	AppName                  string `mapstructure:"app_name"`
	LicenseKey               string `mapstructure:"license_key"`
	Enabled                  bool   `mapstructure:"enabled"`
	LogLevel                 string `mapstructure:"log_level"`
	DistributedTracingEnabled bool  `mapstructure:"distributed_tracing_enabled"`
}

// FirmwareConfig holds the firmware configuration
type FirmwareConfig struct {
	StoragePath       string `mapstructure:"storage_path"`
	KeysPath          string `mapstructure:"keys_path"`
	SigningAlgorithm  string `mapstructure:"signing_algorithm"`
	PublicKeyFile     string `mapstructure:"public_key_file"`
	PrivateKeyFile    string `mapstructure:"private_key_file"`
	VerifySignatures  bool   `mapstructure:"verify_signatures"`
	RequireSignatures bool   `mapstructure:"require_signatures"`
}

// OTAConfig holds the OTA update configuration
type OTAConfig struct {
	ChunkSize            int    `mapstructure:"chunk_size"`
	MaxConcurrentUpdates int    `mapstructure:"max_concurrent_updates"`
	DownloadTimeout      int    `mapstructure:"download_timeout"`
	MaxRetries           int    `mapstructure:"max_retries"`
	SessionLifetime      int    `mapstructure:"session_lifetime"`
	DeltaUpdates         bool   `mapstructure:"delta_updates"`
	DefaultUpdateType    string `mapstructure:"default_update_type"`
}

// LoggingConfig holds the logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// InitConfig initializes the configuration using Viper
func InitConfig(cfgFile string) error {
	v := viper.New()
	
	// Set defaults
	setDefaults(v)
	
	// Use config file from the flag if provided
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		// Search for config in common directories
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/device-service")
		v.SetConfigName("config")
	}
	
	// Set environment variable prefix for config overrides
	v.SetEnvPrefix("DEVICE")
	
	// Enable automatic environment variable binding
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	
	// Read configuration
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, using defaults and environment variables
			fmt.Println("No config file found, using defaults and environment variables")
		} else {
			// Config file was found but another error occurred
			return fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		fmt.Println("Using config file:", v.ConfigFileUsed())
	}
	
	// Set global viper instance (needed because our existing code uses the global viper)
	viper.SetDefault("server", v.GetStringMap("server"))
	viper.SetDefault("database", v.GetStringMap("database"))
	viper.SetDefault("redis", v.GetStringMap("redis"))
	viper.SetDefault("service_bus", v.GetStringMap("service_bus"))
	viper.SetDefault("new_relic", v.GetStringMap("new_relic"))
	viper.SetDefault("firmware", v.GetStringMap("firmware"))
	viper.SetDefault("ota", v.GetStringMap("ota"))
	viper.SetDefault("logging", v.GetStringMap("logging"))
	
	return nil
}

// setDefaults sets default values for configuration
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", 8091)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.timeout", "30s")
	v.SetDefault("server.cors_enabled", true)
	v.SetDefault("server.cors_origins", []string{"*"})
	
	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.name", "device")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 50)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", "1h")
	
	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.enabled", true)
	
	// Service Bus defaults - no default connection string for security
	v.SetDefault("service_bus.queue_name", "device-events")
	
	// New Relic defaults
	v.SetDefault("new_relic.app_name", "Device Service")
	v.SetDefault("new_relic.enabled", false)
	v.SetDefault("new_relic.log_level", "info")
	v.SetDefault("new_relic.distributed_tracing_enabled", true)
	
	// Firmware defaults
	v.SetDefault("firmware.storage_path", "./firmware")
	v.SetDefault("firmware.keys_path", "./keys")
	v.SetDefault("firmware.signing_algorithm", "secp256r1")
	v.SetDefault("firmware.public_key_file", "ecdsa-public.pem")
	v.SetDefault("firmware.private_key_file", "ecdsa-private.pem")
	v.SetDefault("firmware.verify_signatures", true)
	v.SetDefault("firmware.require_signatures", false)
	
	// OTA defaults
	v.SetDefault("ota.chunk_size", 8192)
	v.SetDefault("ota.max_concurrent_updates", 100)
	v.SetDefault("ota.download_timeout", 3600)
	v.SetDefault("ota.max_retries", 3)
	v.SetDefault("ota.session_lifetime", 86400)
	v.SetDefault("ota.delta_updates", false)
	v.SetDefault("ota.default_update_type", "full")

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}

// Load loads the configuration
func Load() (*Config, error) {
	// Make sure viper is initialized
	if viper.GetString("server.mode") == "" {
		if err := InitConfig(""); err != nil {
			return nil, err
		}
	}
	
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to unmarshal config: %w", err)
	}
	
	return &config, nil
}

// GetDSN returns the database connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}
