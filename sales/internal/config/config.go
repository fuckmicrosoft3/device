package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Service       ServiceConfig       `mapstructure:"service"`
	Server        ServerConfig        `mapstructure:"server"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Azure         AzureConfig         `mapstructure:"azure"`
	Elasticsearch ElasticsearchConfig `mapstructure:"elasticsearch"`
	Enrichment    EnrichmentConfig    `mapstructure:"enrichment"`
	Monitoring    MonitoringConfig    `mapstructure:"monitoring"`
	Logging       LoggingConfig       `mapstructure:"logging"`
}

type ServiceConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

type ServerConfig struct {
	HTTP HTTPConfig `mapstructure:"http"`
}

type HTTPConfig struct {
	Address      string        `mapstructure:"address"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
	Sales DatabaseConnection `mapstructure:"sales"`
	ERP   DatabaseConnection `mapstructure:"erp"`
}

type DatabaseConnection struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type AzureConfig struct {
	ServiceBus ServiceBusConfig `mapstructure:"service_bus"`
}

type ServiceBusConfig struct {
	ConnectionString      string `mapstructure:"connection_string"`
	QueueName             string `mapstructure:"queue_name"`
	MaxConcurrentMessages int    `mapstructure:"max_concurrent_messages"`
}

type ElasticsearchConfig struct {
	Addresses   []string `mapstructure:"addresses"`
	Username    string   `mapstructure:"username"`
	Password    string   `mapstructure:"password"`
	IndexPrefix string   `mapstructure:"index_prefix"`
}

type EnrichmentConfig struct {
	Timeout       time.Duration `mapstructure:"timeout"`
	RetryInterval time.Duration `mapstructure:"retry_interval"`
	MaxRetries    int           `mapstructure:"max_retries"`
}

type MonitoringConfig struct {
	NewRelic NewRelicConfig `mapstructure:"new_relic"`
}

type NewRelicConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	AppName    string `mapstructure:"app_name"`
	LicenseKey string `mapstructure:"license_key"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// LoadConfig reads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set config file
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Environment variable settings
	v.SetEnvPrefix("SALES")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Service.Name == "" {
		return fmt.Errorf("service name is required")
	}

	if c.Database.Sales.DSN == "" {
		return fmt.Errorf("sales database DSN is required")
	}

	if c.Database.ERP.DSN == "" {
		return fmt.Errorf("ERP database DSN is required")
	}

	if c.Azure.ServiceBus.ConnectionString == "" {
		return fmt.Errorf("Azure Service Bus connection string is required")
	}

	if len(c.Elasticsearch.Addresses) == 0 {
		return fmt.Errorf("at least one Elasticsearch address is required")
	}

	return nil
}

// GetElasticsearchIndex returns the full index name with prefix
func (c *Config) GetElasticsearchIndex(indexName string) string {
	if c.Elasticsearch.IndexPrefix != "" {
		return fmt.Sprintf("%s-%s", c.Elasticsearch.IndexPrefix, indexName)
	}
	return indexName
}
