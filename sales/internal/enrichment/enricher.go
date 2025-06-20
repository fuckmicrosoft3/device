package enrichment

import (
	"context"
	"fmt"
	"time"

	"example.com/backstage/services/sales/internal/config"
	"example.com/backstage/services/sales/internal/models"
	"example.com/backstage/services/sales/internal/store"

	"github.com/rs/zerolog/log"
)

// Enricher handles ERP data enrichment
type Enricher struct {
	store      store.Store
	config     *config.EnrichmentConfig
	retryQueue *RetryQueue
}

// NewEnricher creates a new enricher instance
func NewEnricher(store store.Store, cfg *config.EnrichmentConfig) *Enricher {
	return &Enricher{
		store:      store,
		config:     cfg,
		retryQueue: NewRetryQueue(cfg.RetryInterval),
	}
}

// Start starts the enrichment workers
func (e *Enricher) Start(ctx context.Context) {
	// Start retry processor
	go e.retryQueue.Start(ctx)

	// Start enrichment worker
	go e.enrichmentWorker(ctx)

	log.Info().Msg("Enrichment service started")
}

// enrichmentWorker continuously processes unenriched sessions
func (e *Enricher) enrichmentWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Enrichment worker stopped")
			return
		case <-ticker.C:
			e.processUnenrichedSessions(ctx)
		}
	}
}

// processUnenrichedSessions fetches and enriches pending sessions
func (e *Enricher) processUnenrichedSessions(ctx context.Context) {
	sessions, err := e.store.GetUnenrichedDispenseSessions(ctx, 100)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch unenriched sessions")
		return
	}

	if len(sessions) == 0 {
		return
	}

	log.Info().Int("count", len(sessions)).Msg("Processing unenriched sessions")

	for _, session := range sessions {
		if err := e.EnrichDispenseSession(ctx, session); err != nil {
			log.Error().
				Err(err).
				Str("session_id", session.ID.String()).
				Msg("Failed to enrich session")
		}
	}
}

// EnrichDispenseSession enriches a dispense session with ERP data
func (e *Enricher) EnrichDispenseSession(ctx context.Context, session *models.DispenseSession) error {
	// Create timeout context
	enrichCtx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	// Update enrichment attempt
	session.EnrichmentAttempts++
	session.LastEnrichmentAt = timePtr(time.Now())

	// Fetch ERP data
	enrichmentData, err := e.fetchERPData(enrichCtx, session)
	if err != nil {
		errMsg := err.Error()
		session.EnrichmentError = &errMsg

		// Update session with error
		if updateErr := e.store.UpdateDispenseSession(ctx, session); updateErr != nil {
			log.Error().Err(updateErr).Msg("Failed to update session after enrichment error")
		}

		// Queue for retry if under max attempts
		if session.EnrichmentAttempts < e.config.MaxRetries {
			e.retryQueue.Add(&RetryItem{
				SessionID:  session.ID,
				RetryAfter: time.Now().Add(e.config.RetryInterval),
				Attempts:   session.EnrichmentAttempts,
			})
		} else {
			// Mark as failed after max attempts
			session.EnrichmentStatus = models.EnrichmentStatusFailed
			e.store.UpdateDispenseSession(ctx, session)
		}

		return fmt.Errorf("failed to fetch ERP data: %w", err)
	}

	// Create sale record with enriched data
	sale := e.buildSaleFromEnrichment(session, enrichmentData)

	// Use transaction to ensure consistency
	err = e.store.WithTransaction(ctx, func(tx store.Store) error {
		// Create sale
		if err := tx.CreateSale(ctx, sale); err != nil {
			return fmt.Errorf("failed to create sale: %w", err)
		}

		// Mark session as enriched and processed
		session.EnrichmentStatus = models.EnrichmentStatusCompleted
		session.IsProcessed = true
		session.EnrichmentError = nil

		if err := tx.UpdateDispenseSession(ctx, session); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Info().
		Str("session_id", session.ID.String()).
		Str("sale_id", sale.ID.String()).
		Msg("Successfully enriched dispense session")

	return nil
}

// fetchERPData fetches all required data from ERP
func (e *Enricher) fetchERPData(ctx context.Context, session *models.DispenseSession) (*models.SaleEnrichmentData, error) {
	if session.DeviceMCU == nil {
		return nil, fmt.Errorf("device MCU is required for enrichment")
	}

	if session.Time == nil {
		return nil, fmt.Errorf("sale time is required for enrichment")
	}

	saleTime := session.GetSaleTime()

	// Fetch device
	device, err := e.store.GetDeviceByMCU(ctx, *session.DeviceMCU)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	// Fetch active device machine revision
	deviceMachineRevision, err := e.store.GetActiveDeviceMachineRevision(ctx, device.ID, saleTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get device machine revision: %w", err)
	}

	// Fetch active machine revision
	machineRevision, err := e.store.GetActiveMachineRevision(ctx, deviceMachineRevision.ID, saleTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get machine revision: %w", err)
	}

	// Fetch machine
	machine, err := e.store.GetMachineByID(ctx, machineRevision.MachineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get machine: %w", err)
	}

	// Fetch tenant
	tenant, err := e.store.GetTenantByID(ctx, machineRevision.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	// Fetch location (optional)
	location, _ := e.store.GetMachineLocationByMachineID(ctx, machine.ID)

	// Fetch position (optional)
	position, _ := e.store.GetPositionByMachineID(ctx, machine.ID)

	return &models.SaleEnrichmentData{
		Device:          device,
		MachineRevision: machineRevision,
		Machine:         machine,
		Location:        location,
		Position:        position,
		Tenant:          tenant,
	}, nil
}

// buildSaleFromEnrichment creates a sale record from enrichment data
func (e *Enricher) buildSaleFromEnrichment(session *models.DispenseSession, data *models.SaleEnrichmentData) *models.Sale {
	saleType := models.DetermineSaleType(&session.AmountKSH, nil)

	sale := &models.Sale{
		DispenseSessionID: session.ID,
		MachineRevisionID: data.MachineRevision.ID,
		MachineID:         data.Machine.ID,
		TenantID:          data.Tenant.ID,
		Type:              saleType,
		Amount:            session.AmountKSH,
		Quantity:          1,
		Time:              session.GetSaleTime(),
		IsValid:           true,
		IsReconciled:      false,
	}

	// Set optional fields
	if data.Position != nil {
		sale.Position = data.Position.Position
		sale.ProductID = data.Position.ProductID
	}

	if data.Machine.OrganizationID != nil {
		sale.OrganizationID = data.Machine.OrganizationID
	}

	return sale
}

// ProcessRetryQueue processes items from the retry queue
func (e *Enricher) ProcessRetryQueue(ctx context.Context) error {
	items := e.retryQueue.GetDueItems()

	for _, item := range items {
		session, err := e.store.GetDispenseSessionByID(ctx, item.SessionID)
		if err != nil {
			log.Error().
				Err(err).
				Str("session_id", item.SessionID.String()).
				Msg("Failed to fetch session for retry")
			continue
		}

		if session.EnrichmentStatus == models.EnrichmentStatusCompleted {
			// Already enriched, skip
			continue
		}

		if err := e.EnrichDispenseSession(ctx, session); err != nil {
			log.Error().
				Err(err).
				Str("session_id", session.ID.String()).
				Msg("Retry enrichment failed")
		}
	}

	return nil
}

// GetRetryQueueStats returns retry queue statistics
func (e *Enricher) GetRetryQueueStats() map[string]interface{} {
	return e.retryQueue.GetStats()
}

// Helper function
func timePtr(t time.Time) *time.Time {
	return &t
}
