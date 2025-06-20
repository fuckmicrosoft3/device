package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/backstage/services/sales/internal/models"
	"example.com/backstage/services/sales/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// DispensePayloadRequest represents the incoming dispense payload
type DispensePayloadRequest struct {
	Amount             int32     `json:"a" binding:"required"`
	AVol               int       `json:"a_vol"`
	DVol               int       `json:"d_vol"`
	Device             string    `json:"device" binding:"required"`
	Dt                 int       `json:"dt"`
	InterpolatedVolume int       `json:"e_vol"`
	EventType          string    `json:"ev"`
	ID                 string    `json:"id"`
	Ms                 int       `json:"ms"`
	Product            int       `json:"p"`
	RVol               float64   `json:"r_vol"`
	S                  int       `json:"s"`
	SaleTime           int       `json:"t" binding:"required"`
	Tag                string    `json:"tag"`
	Timestamp          string    `json:"timestamp"`
	Topic              string    `json:"topic"`
	IdempotencyKey     uuid.UUID `json:"u"`
}

// DispensePayloadResponse represents the response for dispense payload
type DispensePayloadResponse struct {
	Success           bool      `json:"success"`
	Message           string    `json:"message"`
	DispenseSessionID uuid.UUID `json:"dispense_session_id"`
	Timestamp         time.Time `json:"timestamp"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// handleDispensePayload handles incoming dispense payloads
func (s *Server) handleDispensePayload(c *gin.Context) {
	var request DispensePayloadRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Create dispense session
	saleTime := int32(request.SaleTime)
	session := &models.DispenseSession{
		EventType:        request.EventType,
		ExpectedDispense: float64(request.InterpolatedVolume),
		RemainingVolume:  request.RVol,
		AmountKSH:        request.Amount,
		IdempotencyKey:   request.IdempotencyKey,
		Time:             &saleTime,
		DeviceMCU:        &request.Device,
		EnrichmentStatus: models.EnrichmentStatusPending,
	}

	// Use idempotency cache
	result, err := s.idempotencyCache.Get(request.IdempotencyKey)
	if err == nil && result != nil {
		// Return cached result
		if response, ok := result.(*DispensePayloadResponse); ok {
			c.JSON(http.StatusOK, response)
			return
		}
	}

	// Process the request
	ctx := c.Request.Context()
	err = s.store.CreateDispenseSession(ctx, session)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateKey) {
			// Handle duplicate by fetching existing session
			existingSession, fetchErr := s.store.GetDispenseSessionByIdempotencyKey(ctx, request.IdempotencyKey)
			if fetchErr == nil {
				response := &DispensePayloadResponse{
					Success:           true,
					Message:           "Duplicate request - returning existing session",
					DispenseSessionID: existingSession.ID,
					Timestamp:         existingSession.CreatedAt,
				}
				c.JSON(http.StatusOK, response)
				return
			}
		}

		log.Error().Err(err).Msg("Failed to create dispense session")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to process request",
			Message: "Internal server error",
		})
		return
	}

	// Queue for enrichment asynchronously
	go func() {
		enrichCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.enricher.EnrichDispenseSession(enrichCtx, session); err != nil {
			log.Error().
				Err(err).
				Str("session_id", session.ID.String()).
				Msg("Failed to enrich dispense session")
		} else {
			// Index the sale if enrichment succeeded
			sale, err := s.store.GetSaleByDispenseSessionID(enrichCtx, session.ID)
			if err == nil {
				s.indexer.IndexSale(sale.ID)
			}
		}
	}()

	// Create response
	response := &DispensePayloadResponse{
		Success:           true,
		Message:           "Sale payload received",
		DispenseSessionID: session.ID,
		Timestamp:         session.CreatedAt,
	}

	// Cache the response
	s.idempotencyCache.Set(request.IdempotencyKey, response, 5*time.Minute)

	c.JSON(http.StatusCreated, response)
}

// handleGetSale retrieves a sale by ID
func (s *Server) handleGetSale(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid ID",
			Message: "ID must be a valid UUID",
		})
		return
	}

	ctx := c.Request.Context()
	sale, err := s.store.GetSaleByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error: "Sale not found",
			})
			return
		}

		log.Error().Err(err).Msg("Failed to fetch sale")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to fetch sale",
		})
		return
	}

	c.JSON(http.StatusOK, sale)
}

// handleStats returns service statistics
func (s *Server) handleStats(c *gin.Context) {
	stats := map[string]interface{}{
		"service": map[string]interface{}{
			"name":        s.config.Service.Name,
			"environment": s.config.Service.Environment,
			"uptime":      time.Since(startTime).String(),
		},
		"enrichment": map[string]interface{}{
			"retry_queue": s.enricher.GetRetryQueueStats(),
		},
		"indexer": s.indexer.GetStats(),
	}

	c.JSON(http.StatusOK, stats)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now(),
	})
}

// handleReady handles readiness check requests
func (s *Server) handleReady(c *gin.Context) {
	ctx := c.Request.Context()

	// Check database connectivity
	if err := s.store.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"time":   time.Now(),
	})
}

// Package variable to track start time
var startTime = time.Now()
