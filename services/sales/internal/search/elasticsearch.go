package search

import (
	"bytes"
	"context"
	"encoding/json"
	"example.com/backstage/services/sales/config"
	"example.com/backstage/services/sales/internal/models"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// ElasticClient provides integration with Elasticsearch
type ElasticClient struct {
	client *elasticsearch.Client
	config config.ElasticConfig
}

// NewElasticClient creates a new Elasticsearch client
func NewElasticClient(cfg config.ElasticConfig) (*ElasticClient, error) {
	esConfig := elasticsearch.Config{
		Addresses: []string{cfg.URL},
		Username:  cfg.Username,
		Password:  cfg.Password,
	}

	client, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Elasticsearch client")
	}

	return &ElasticClient{
		client: client,
		config: cfg,
	}, nil
}

// IndexSale indexes a sale in Elasticsearch
func (c *ElasticClient) IndexSale(ctx context.Context, sale *models.Sale, machine *models.Machine, machineLocation string) error {
	log.Info().Str("sale_id", sale.ID.String()).Msg("indexing sale")

	// Extract machine attributes
	var machineAttributes []map[string]interface{}
	if machine.Attributes != nil {
		if err := json.Unmarshal(machine.Attributes, &machineAttributes); err != nil {
			log.Error().Err(err).Msg("could not unmarshal machine attributes")
			return errors.Wrap(err, "failed to unmarshal machine attributes")
		}
	}

	// Build the document to be indexed
	saleDoc := map[string]interface{}{
		"id":                  sale.ID.String(),
		"time":                sale.Time,
		"type":                sale.Type,
		"amount":              sale.Amount,
		"quantity":            sale.Quantity,
		"position":            sale.Position,
		"machine_revision_id": sale.MachineRevisionID.String(),
		"machine_id":          sale.MachineID.String(),
		"machine_name":        machine.Name,
		"machine_service_tag": machine.ServiceTag,
		"machine_serial_tag":  machine.SerialTag,
		"machine_location":    machineLocation,
		"dispense_session_id": sale.DispenseSessionID.String(),
	}

	// Add machine attributes to the document
	for _, attr := range machineAttributes {
		if key, ok := attr["key"].(string); ok {
			if values, ok := attr["values"].([]interface{}); ok {
				saleDoc["attributes:"+key] = values
			}
		}
	}

	// Marshall the document to JSON
	docJson, err := json.Marshal(saleDoc)
	if err != nil {
		return errors.Wrap(err, "failed to marshal sale document")
	}

	// Prepare the index request
	indexName := config.FormatIndex(c.config, c.config.Index)
	req := esapi.IndexRequest{
		Index:      indexName,
		DocumentID: sale.DispenseSessionID.String(),
		Body:       bytes.NewReader(docJson),
		Refresh:    "true",
	}

	// Execute the request
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return errors.Wrap(err, "failed to execute Elasticsearch index request")
	}
	defer res.Body.Close()

	// Check for errors in the response
	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return errors.Wrap(err, "failed to parse Elasticsearch error response")
		}
		return errors.Errorf("Elasticsearch index error: %v", e)
	}

	log.Info().Str("sale_id", sale.ID.String()).Msg("sale indexed successfully")
	return nil
}

// SearchSales searches for sales with the given criteria
func (c *ElasticClient) SearchSales(ctx context.Context, query map[string]interface{}) ([]map[string]interface{}, error) {
	// Convert query to JSON
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal search query")
	}

	// Prepare the search request
	indexName := config.FormatIndex(c.config, c.config.Index)
	req := esapi.SearchRequest{
		Index: []string{indexName},
		Body:  bytes.NewReader(queryJSON),
	}

	// Execute the request
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute Elasticsearch search request")
	}
	defer res.Body.Close()

	// Check for errors in the response
	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return nil, errors.Wrap(err, "failed to parse Elasticsearch error response")
		}
		return nil, errors.Errorf("Elasticsearch search error: %v", e)
	}

	// Parse the response
	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, errors.Wrap(err, "failed to parse Elasticsearch search response")
	}

	// Extract the hits
	hits, ok := result["hits"].(map[string]interface{})
	if !ok {
		return nil, errors.New("unexpected search result format")
	}

	hitsArray, ok := hits["hits"].([]interface{})
	if !ok {
		return nil, errors.New("unexpected hits format")
	}

	// Extract the documents
	var docs []map[string]interface{}
	for _, hit := range hitsArray {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}

		docs = append(docs, source)
	}

	return docs, nil
}