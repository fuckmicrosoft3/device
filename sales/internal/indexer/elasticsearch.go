package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"example.com/backstage/services/sales/internal/config"
	"example.com/backstage/services/sales/internal/models"
	"example.com/backstage/services/sales/internal/store"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Indexer handles ElasticSearch indexing operations
type Indexer struct {
	client     *elasticsearch.Client
	store      store.Store
	config     *config.ElasticsearchConfig
	indexQueue chan *IndexRequest
	wg         sync.WaitGroup
}

// IndexRequest represents a request to index a sale
type IndexRequest struct {
	SaleID    uuid.UUID
	Timestamp time.Time
	Retries   int
}

// NewIndexer creates a new ElasticSearch indexer
func NewIndexer(cfg *config.Config, store store.Store) (*Indexer, error) {
	// Configure ElasticSearch client
	esConfig := elasticsearch.Config{
		Addresses: cfg.Elasticsearch.Addresses,
		Username:  cfg.Elasticsearch.Username,
		Password:  cfg.Elasticsearch.Password,
	}

	client, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	// Test connection
	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch error: %s", res.String())
	}

	indexer := &Indexer{
		client:     client,
		store:      store,
		config:     &cfg.Elasticsearch,
		indexQueue: make(chan *IndexRequest, 1000),
	}

	// Create index if it doesn't exist
	if err := indexer.createIndex(); err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}

	return indexer, nil
}

// Start starts the indexing workers
func (i *Indexer) Start(ctx context.Context, workers int) {
	for w := 0; w < workers; w++ {
		i.wg.Add(1)
		go i.worker(ctx, w)
	}

	log.Info().Int("workers", workers).Msg("ElasticSearch indexer started")
}

// Stop gracefully stops the indexer
func (i *Indexer) Stop() {
	close(i.indexQueue)
	i.wg.Wait()
	log.Info().Msg("ElasticSearch indexer stopped")
}

// IndexSale queues a sale for indexing
func (i *Indexer) IndexSale(saleID uuid.UUID) {
	request := &IndexRequest{
		SaleID:    saleID,
		Timestamp: time.Now(),
		Retries:   0,
	}

	select {
	case i.indexQueue <- request:
		log.Debug().Str("sale_id", saleID.String()).Msg("Sale queued for indexing")
	default:
		log.Warn().Str("sale_id", saleID.String()).Msg("Index queue full, dropping request")
	}
}

// worker processes index requests
func (i *Indexer) worker(ctx context.Context, id int) {
	defer i.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case request, ok := <-i.indexQueue:
			if !ok {
				return
			}

			if err := i.processIndexRequest(ctx, request); err != nil {
				log.Error().
					Err(err).
					Str("sale_id", request.SaleID.String()).
					Int("worker", id).
					Msg("Failed to index sale")

				// Retry logic
				if request.Retries < 3 {
					request.Retries++
					time.Sleep(time.Duration(request.Retries) * time.Second)

					select {
					case i.indexQueue <- request:
					default:
						log.Warn().Msg("Failed to requeue index request")
					}
				}
			}
		}
	}
}

// processIndexRequest processes a single index request
func (i *Indexer) processIndexRequest(ctx context.Context, request *IndexRequest) error {
	// Fetch sale with relationships
	sale, err := i.store.GetSaleByID(ctx, request.SaleID)
	if err != nil {
		return fmt.Errorf("failed to fetch sale: %w", err)
	}

	// Build document
	doc := i.buildDocument(sale)

	// Marshal to JSON
	jsonDoc, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	// Index document
	indexName := i.config.GetElasticsearchIndex("sales")
	req := esapi.IndexRequest{
		Index:      indexName,
		DocumentID: sale.ID.String(),
		Body:       bytes.NewReader(jsonDoc),
		Refresh:    "false", // Don't wait for refresh
	}

	res, err := req.Do(ctx, i.client)
	if err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return fmt.Errorf("elasticsearch error: %s", res.Status())
		}
		return fmt.Errorf("elasticsearch error: %v", e)
	}

	log.Debug().
		Str("sale_id", sale.ID.String()).
		Str("index", indexName).
		Msg("Sale indexed successfully")

	return nil
}

// buildDocument builds the ElasticSearch document
func (i *Indexer) buildDocument(sale *models.Sale) map[string]interface{} {
	doc := map[string]interface{}{
		"id":                  sale.ID.String(),
		"dispense_session_id": sale.DispenseSessionID.String(),
		"time":                sale.Time,
		"type":                sale.Type,
		"amount":              sale.Amount,
		"quantity":            sale.Quantity,
		"position":            sale.Position,
		"machine_revision_id": sale.MachineRevisionID.String(),
		"machine_id":          sale.MachineID.String(),
		"tenant_id":           sale.TenantID.String(),
		"is_reconciled":       sale.IsReconciled,
		"is_valid":            sale.IsValid,
		"created_at":          sale.CreatedAt,
		"updated_at":          sale.UpdatedAt,
	}

	// Add machine details if available
	if sale.Machine != nil {
		doc["machine_name"] = sale.Machine.Name
		doc["machine_service_tag"] = sale.Machine.ServiceTag
		doc["machine_serial_tag"] = sale.Machine.SerialTag

		// Add machine attributes
		if sale.Machine.Attributes != nil {
			doc["machine_attributes"] = sale.Machine.Attributes
		}
	}

	// Add optional fields
	if sale.ProductID != nil {
		doc["product_id"] = sale.ProductID.String()
	}

	if sale.OrganizationID != nil {
		doc["organization_id"] = sale.OrganizationID.String()
	}

	if sale.TransactionID != nil {
		doc["transaction_id"] = sale.TransactionID.String()
	}

	return doc
}

// createIndex creates the ElasticSearch index with mappings
func (i *Indexer) createIndex() error {
	indexName := i.config.GetElasticsearchIndex("sales")

	// Check if index exists
	exists, err := i.indexExists(indexName)
	if err != nil {
		return err
	}

	if exists {
		log.Debug().Str("index", indexName).Msg("Index already exists")
		return nil
	}

	// Create index with mappings
	mapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":                  map[string]interface{}{"type": "keyword"},
				"dispense_session_id": map[string]interface{}{"type": "keyword"},
				"time":                map[string]interface{}{"type": "date"},
				"type":                map[string]interface{}{"type": "keyword"},
				"amount":              map[string]interface{}{"type": "integer"},
				"quantity":            map[string]interface{}{"type": "integer"},
				"position":            map[string]interface{}{"type": "integer"},
				"machine_revision_id": map[string]interface{}{"type": "keyword"},
				"machine_id":          map[string]interface{}{"type": "keyword"},
				"tenant_id":           map[string]interface{}{"type": "keyword"},
				"product_id":          map[string]interface{}{"type": "keyword"},
				"organization_id":     map[string]interface{}{"type": "keyword"},
				"transaction_id":      map[string]interface{}{"type": "keyword"},
				"is_reconciled":       map[string]interface{}{"type": "boolean"},
				"is_valid":            map[string]interface{}{"type": "boolean"},
				"machine_name":        map[string]interface{}{"type": "text"},
				"machine_service_tag": map[string]interface{}{"type": "keyword"},
				"machine_serial_tag":  map[string]interface{}{"type": "keyword"},
				"machine_attributes":  map[string]interface{}{"type": "object"},
				"created_at":          map[string]interface{}{"type": "date"},
				"updated_at":          map[string]interface{}{"type": "date"},
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(mapping); err != nil {
		return fmt.Errorf("failed to encode mapping: %w", err)
	}

	res, err := i.client.Indices.Create(
		indexName,
		i.client.Indices.Create.WithBody(&buf),
	)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to create index: %s", res.String())
	}

	log.Info().Str("index", indexName).Msg("Created ElasticSearch index")
	return nil
}

// indexExists checks if an index exists
func (i *Indexer) indexExists(indexName string) (bool, error) {
	res, err := i.client.Indices.Exists([]string{indexName})
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	return res.StatusCode == 200, nil
}

// GetStats returns indexer statistics
func (i *Indexer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"queue_size": len(i.indexQueue),
		"queue_cap":  cap(i.indexQueue),
	}
}
