package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Environment    string        `mapstructure:"environment"`
	ServerAddress  string        `mapstructure:"server.address"`
	ServerTimeout  time.Duration `mapstructure:"server.timeout"`
	CorsEnabled    bool          `mapstructure:"server.cors_enabled"`
	CorsOrigins    []string      `mapstructure:"server.cors_origins"`
	MetricsEnabled bool          `mapstructure:"metrics_enabled"`
	LogLevel       string        `mapstructure:"logging.level"`
	LogFormat      string        `mapstructure:"logging.format"`
	DB             DatabaseConfig
	Redis          RedisConfig
	Azure          AzureConfig
	Elastic        ElasticConfig
	Tracing        TracingConfig
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	DSN             string        `mapstructure:"database.dsn"`
	ReadOnlyDSN     string        `mapstructure:"database.read_only_dsn"`
	Name            string        `mapstructure:"database.name"`
	MaxOpenConns    int           `mapstructure:"database.max_open_conns"`
	MaxIdleConns    int           `mapstructure:"database.max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"database.conn_max_lifetime"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string `mapstructure:"redis.host"`
	Port     int    `mapstructure:"redis.port"`
	Password string `mapstructure:"redis.password"`
	DB       int    `mapstructure:"redis.db"`
	Enabled  bool   `mapstructure:"redis.enabled"`
}

// AzureConfig holds Azure Service Bus configuration
type AzureConfig struct {
	QueueConnStr string `mapstructure:"azure.queue_conn_str"`
	QueueName    string `mapstructure:"azure.queue_name"`
}

// ElasticConfig holds Elasticsearch configuration
type ElasticConfig struct {
	URL      string `mapstructure:"elastic.url"`
	Username string `mapstructure:"elastic.username"`
	Password string `mapstructure:"elastic.password"`
	Prefix   string `mapstructure:"elastic.prefix"`
	Index    string `mapstructure:"elastic.index"`
}

// TracingConfig holds tracing configuration
type TracingConfig struct {
	LicenseKey     string `mapstructure:"tracing.license_key"`
	AppName        string `mapstructure:"tracing.app_name"`
	LogLevel       string `mapstructure:"tracing.log_level"`
	LogEnabled     bool   `mapstructure:"tracing.log_enabled"`
	DistribTracing bool   `mapstructure:"tracing.distributed_tracing_enabled"`
}

// LoadConfig reads configuration from file or environment variables
func LoadConfig(path string) (Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Setup configuration paths
	v.AddConfigPath(path)
	v.AddConfigPath("./config")
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	// Try to read the YAML config first
	if err := v.ReadInConfig(); err != nil {
		// If YAML not found, try ENV file
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			v.SetConfigName("app")
			v.SetConfigType("env")
			if err := v.ReadInConfig(); err != nil {
				// Continue even if no config file is found - we'll use ENV vars and defaults
				fmt.Printf("Warning: No configuration file found: %v\n", err)
			}
		} else {
			// Return if there's an error reading the found config file
			return Config{}, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Enable environment variables to override config
	v.SetEnvPrefix("SALES")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return Config{}, fmt.Errorf("unable to unmarshal config: %w", err)
	}

	return config, nil
}

// setDefaults sets default values for configuration
func setDefaults(v *viper.Viper) {
	// Core settings
	v.SetDefault("environment", "development")
	v.SetDefault("server.address", "0.0.0.0:8080")
	v.SetDefault("server.timeout", "30s")
	v.SetDefault("server.cors_enabled", true)
	v.SetDefault("server.cors_origins", []string{"*"})
	v.SetDefault("metrics_enabled", true)

	// Database settings
	v.SetDefault("database.dsn", "postgresql://postgres:postgres@localhost:5432/sales?sslmode=disable")
	v.SetDefault("database.read_only_dsn", "postgresql://postgres:postgres@localhost:5432/sales_readonly?sslmode=disable")
	v.SetDefault("database.name", "sales")
	v.SetDefault("database.max_open_conns", 50)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", "1h")

	// Redis settings
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.enabled", true)

	// Azure settings
	v.SetDefault("azure.queue_name", "sales-events")

	// Elasticsearch settings
	v.SetDefault("elastic.url", "http://localhost:9200")
	v.SetDefault("elastic.prefix", "sales")
	v.SetDefault("elastic.index", "sales")

	// Tracing settings
	v.SetDefault("tracing.app_name", "Sales Service")
	v.SetDefault("tracing.log_level", "info")
	v.SetDefault("tracing.log_enabled", true)
	v.SetDefault("tracing.distributed_tracing_enabled", true)

	// Logging settings
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}

// FormatIndex formats an Elasticsearch index name with the configured prefix
func FormatIndex(cfg ElasticConfig, index string) string {
	return cfg.Prefix + "-" + index
}
