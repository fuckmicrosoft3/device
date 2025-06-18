package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"go.novek.io/canister/handlers"
	"go.novek.io/canister/models"
)

// CanisterAggregateResponse is the response for a canister aggregate
type CanisterAggregateResponse struct {
	ID                 string                 `json:"id"`
	Version            int                    `json:"version"`
	AggregateID        string                 `json:"aggregate_id"`
	Tag                string                 `json:"tag"`
	MCU                string                 `json:"mcu"`
	Model              string                 `json:"model"`
	Name               string                 `json:"name"`
	OrganisationID     string                 `json:"organisation_id"`
	Status             string                 `json:"status"`
	Attributes         map[string]interface{} `json:"attributes"`
	LastMovementID     string                 `json:"last_movement_id"`
	CurrentTemperature float64                `json:"current_temperature"`
	CurrentVolume      float64                `json:"current_volume"`
	TamperState        string                 `json:"tamper_state"`
	TamperSources      []string               `json:"tamper_sources"`
}

// CanisterEventRequest is the request for a canister event
type CanisterEventRequest struct {
	EventType string          `json:"eventType"`
	Data      json.RawMessage `json:"data"`
}

// getCanisterAggregate returns a canister aggregate by ID
func (s *Server) getCanisterAggregate(c *gin.Context) {
	id := c.Param("id")

	canister := s.db.Model(&models.Canister{}).Where("aggregate_id = ?", id).First(&models.Canister{}).Row()

	if canister == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "canister not found"})
		return
	}

	// Convert to response
	var response CanisterAggregateResponse
	// Populate response fields from canister

	c.JSON(http.StatusOK, response)
}

// receiveCanisterEvents processes canister events
func (s *Server) receiveCanisterEvents(c *gin.Context) {
	var req CanisterEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch req.EventType {
	case "CreateCanister":
		var cmd handlers.CreateCanisterCommand
		if err := json.Unmarshal(req.Data, &cmd); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// If AggregateID is not provided, generate a new one
		if cmd.AggregateID == "" {
			cmd.AggregateID = uuid.New().String()
		}

		if err := s.canisterHandler.HandleCreateCanister(ctx, cmd); err != nil {
			log.Error().Err(err).Msg("Failed to handle CreateCanister command")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	case "UpdateCanister":
		var cmd handlers.UpdateCanisterCommand
		if err := json.Unmarshal(req.Data, &cmd); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := s.canisterHandler.HandleUpdateCanister(ctx, cmd); err != nil {
			log.Error().Err(err).Msg("Failed to handle UpdateCanister command")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	case "CanisterDamage":
		var cmd handlers.CanisterDamageCommand
		if err := json.Unmarshal(req.Data, &cmd); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := s.canisterHandler.HandleCanisterDamage(ctx, cmd); err != nil {
			log.Error().Err(err).Msg("Failed to handle CanisterDamage command")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	// Add other event types...

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported event type"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "event processed successfully"})
}

// getOrgCanistersForDistribution returns canisters for distribution
func (s *Server) getOrgCanistersForDistribution(c *gin.Context) {
	orgID := c.Query("organisation_id")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organisation_id is required"})
		return
	}

	var canisters []models.Canister
	if err := s.db.Where("organisation_id = ? AND status = ?", orgID, "ReadyForUse").Find(&canisters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"canisters": canisters})
}
