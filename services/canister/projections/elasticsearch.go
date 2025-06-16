package projections

import (
	"fmt"
	"net/http"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/rs/zerolog/log"

	"example.com/backstage/services/canister/config"
)

// NewElasticsearchClient creates a new Elasticsearch client
func NewElasticsearchClient(cfg config.Config) (*elasticsearch.Client, error) {
	elasticCfg := elasticsearch.Config{
		Addresses: []string{cfg.ElasticSearchURL},
		Username:  cfg.ElasticSearchUsername,
		Password:  cfg.ElasticSearchPassword,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
		},
	}

	client, err := elasticsearch.NewClient(elasticCfg)
	if err != nil {
		return nil, fmt.Errorf("error creating Elasticsearch client: %w", err)
	}

	// Check the connection
	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("error connecting to Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("Elasticsearch returned error: %s", res.String())
	}

	log.Info().Msg("Successfully connected to Elasticsearch")
	return client, nil
}

// FormatIndex adds the prefix to the index name
func FormatIndex(indexName string, cfg config.Config) string {
	return cfg.ElasticSearchPrefix + "-" + indexName
}

// EnsureIndices ensures that all required indices exist
func EnsureIndices(client *elasticsearch.Client, cfg config.Config) error {
	indices := []string{
		"canisters",
		"canister-events",
		"canister-movements",
		"canister-refills-sessions",
	}

	for _, index := range indices {
		formattedIndex := FormatIndex(index, cfg)
		
		// Check if the index exists
		exists, err := indexExists(client, formattedIndex)
		if err != nil {
			return err
		}

		if !exists {
			log.Info().Msgf("Creating index %s", formattedIndex)
			if err := createIndex(client, formattedIndex); err != nil {
				return err
			}
		}
	}

	return nil
}

// indexExists checks if an index exists
func indexExists(client *elasticsearch.Client, index string) (bool, error) {
	res, err := client.Indices.Exists([]string{index})
	if err != nil {
		return false, fmt.Errorf("error checking if index %s exists: %w", index, err)
	}
	defer res.Body.Close()

	return res.StatusCode == 200, nil
}

// createIndex creates an index
func createIndex(client *elasticsearch.Client, index string) error {
	res, err := client.Indices.Create(index)
	if err != nil {
		return fmt.Errorf("error creating index %s: %w", index, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error creating index %s: %s", index, res.String())
	}

	return nil
}