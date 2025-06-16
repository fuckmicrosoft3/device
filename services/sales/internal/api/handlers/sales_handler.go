package handlers

import (
	"net/http"
	"example.com/backstage/services/sales/internal/models"
	"example.com/backstage/services/sales/internal/services"
	"example.com/backstage/services/sales/internal/tracing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// SalesHandler handles sales-related HTTP requests
type SalesHandler struct {
	salesService *services.SalesService
	tracer       tracing.Tracer
}

// NewSalesHandler creates a new sales handler
func NewSalesHandler(salesService *services.SalesService, tracer tracing.Tracer) *SalesHandler {
	return &SalesHandler{
		salesService: salesService,
		tracer:       tracer,
	}
}

// SalePayloadRequest represents an incoming sale payload request
type SalePayloadRequest struct {
	Amount         int32     `json:"a" binding:"required"`
	AVol           int       `json:"a_vol"`
	DVol           int       `json:"d_vol"`
	Device         string    `json:"device" binding:"required"`
	Dt             int       `json:"dt"`
	EVol           float64   `json:"e_vol"`
	EventType      string    `json:"ev"`
	ID             string    `json:"id"`
	Ms             int       `json:"ms"`
	Product        int       `json:"p"`
	RVol           float64   `json:"r_vol"`
	S              int       `json:"s"`
	SaleTime       int32     `json:"t" binding:"required"`
	Tag            string    `json:"tag"`
	Timestamp      string    `json:"timestamp"`
	Topic          string    `json:"topic"`
	IdempotencyKey uuid.UUID `json:"u"`
}

// SalePayloadResponse represents a response to a sale payload request
type SalePayloadResponse struct {
	DispenseSessionID uuid.UUID `json:"dispense_session_id"`
	Success           bool      `json:"success"`
	Message           string    `json:"message"`
	Timestamp         string    `json:"timestamp"`
}

// HandleIncomingSalePayload handles an incoming sale payload
func (h *SalesHandler) HandleIncomingSalePayload(c *gin.Context) {
	// Start transaction
	txn := h.tracer.StartTransaction("api-incoming-sale-payload")
	defer h.tracer.EndTransaction(txn)

	// Parse the request
	var req SalePayloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error().Err(err).Msg("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		h.tracer.RecordError(txn, err)
		return
	}

	// Add request details to transaction
	h.tracer.AddAttribute(txn, "device", req.Device)
	h.tracer.AddAttribute(txn, "amount", req.Amount)
	h.tracer.AddAttribute(txn, "sale_time", req.SaleTime)

	// Generate idempotency key if not provided
	if req.IdempotencyKey == uuid.Nil {
		req.IdempotencyKey = uuid.New()
	}

	// Convert to service model
	payload := &models.SalePayload{
		Amount:          req.Amount,
		AVol:            req.AVol,
		DVol:            req.DVol,
		Device:          req.Device,
		Dt:              req.Dt,
		EVol:            req.EVol,
		EventType:       req.EventType,
		Ms:              req.Ms,
		P:               req.Product,
		RemainingVolume: req.RVol,
		S:               req.S,
		Time:            req.SaleTime,
		Tag:             req.Tag,
		IdempotencyKey:  req.IdempotencyKey,
	}

	// If event type is not set, default to "dispense"
	if payload.EventType == "" {
		payload.EventType = "dispense"
	}

	// Create dispense session (which now handles sale creation internally)
	dispenseSession, err := h.salesService.CreateDispenseSession(c, payload)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create dispense session")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		h.tracer.RecordError(txn, err)
		return
	}

	// Prepare response
	timestamp := req.Timestamp
	if timestamp == "" {
		timestamp = time.Now().Format(time.RFC3339)
	}

	// Check if the session was marked as processed 
	// If so, we can indicate that the sale was created as well
	message := "Sale payload received"
	if dispenseSession.IsProcessed {
		message = "Sale payload received and processed"
	}

	response := SalePayloadResponse{
		Success:           true,
		Message:           message,
		Timestamp:         timestamp,
		DispenseSessionID: dispenseSession.ID,
	}

	c.JSON(http.StatusCreated, response)
}

// RegisterRoutes registers the handler's routes
func (h *SalesHandler) RegisterRoutes(router *gin.Engine) {
	router.POST("/receive_sale_payload", h.HandleIncomingSalePayload)
}