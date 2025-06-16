package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

var configFile string

type Config struct {
	// Database
	DBDriver string `mapstructure:"database.driver"`
	DBSource string `mapstructure:"database.source"`
	
	// HTTP Server
	HTTPServerAddress string        `mapstructure:"server.address"`
	HTTPServerTimeout time.Duration `mapstructure:"server.timeout"`
	CorsEnabled       bool          `mapstructure:"server.cors_enabled"`
	CorsOrigins       []string      `mapstructure:"server.cors_origins"`
	
	// Elasticsearch
	ElasticSearchURL      string `mapstructure:"elasticsearch.url"`
	ElasticSearchUsername string `mapstructure:"elasticsearch.username"`
	ElasticSearchPassword string `mapstructure:"elasticsearch.password"`
	ElasticSearchPrefix   string `mapstructure:"elasticsearch.prefix"`
	
	// Azure Service Bus
	AzureQueueConnStr                   string `mapstructure:"azure.queue_conn_str"`
	AzureMessagesConfigurationQueueName string `mapstructure:"azure.messages_conf_queue_name"`
	AzureMessagesEventsQueueName        string `mapstructure:"azure.messages_events_queue_name"`
	
	// IAM
	IAMServerAddress string `mapstructure:"iam.server_address"`
	
	// Other configuration
	SnapshotFrequency int  `mapstructure:"snapshot_frequency"`
	EnableMigrations  bool `mapstructure:"enable_migrations"`

	// Logging
	LogLevel  string `mapstructure:"logging.level"`
	LogFormat string `mapstructure:"logging.format"`
}

func SetConfigFile(file string) {
	configFile = file
}

func LoadConfig() (Config, error) {
	var config Config

	viper.SetConfigType("yaml")

	// Set defaults
	setDefaults()

	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.SetConfigName("config")
	}

	// Handle environment variables
	viper.SetEnvPrefix("CANISTER")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		// Try app.env file if yaml not found
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			viper.SetConfigType("env")
			viper.SetConfigName("app")
			if err := viper.ReadInConfig(); err != nil {
				return config, fmt.Errorf("error loading configuration: %w", err)
			}
		} else {
			return config, fmt.Errorf("error loading configuration: %w", err)
		}
	}

	if err := viper.Unmarshal(&config); err != nil {
		return config, fmt.Errorf("error unmarshaling configuration: %w", err)
	}

	return config, nil
}

// FormatIndex adds the configured prefix to an index name
func FormatIndex(config Config, index string) string {
	return config.ElasticSearchPrefix + "-" + index
}

// Set default configuration values
func setDefaults() {
	// Database
	viper.SetDefault("database.driver", "postgres")
	viper.SetDefault("database.source", "postgresql://postgres:postgres@localhost:5432/canister?sslmode=disable")
	
	// HTTP Server
	viper.SetDefault("server.address", "0.0.0.0:8080")
	viper.SetDefault("server.timeout", "30s")
	viper.SetDefault("server.cors_enabled", true)
	viper.SetDefault("server.cors_origins", []string{"*"})
	
	// Elasticsearch
	viper.SetDefault("elasticsearch.url", "http://localhost:9200")
	viper.SetDefault("elasticsearch.prefix", "canister")
	
	// Azure Service Bus
	viper.SetDefault("azure.messages_conf_queue_name", "canister-configurations")
	viper.SetDefault("azure.messages_events_queue_name", "canister-events")
	
	// Other configuration
	viper.SetDefault("snapshot_frequency", 100)
	viper.SetDefault("enable_migrations", true)

	// Logging
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
}