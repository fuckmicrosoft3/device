package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"example.com/backstage/services/canister/handlers"
)

// DeliveryEventRequest is the request for a delivery event
type DeliveryEventRequest struct {
	EventType string          `json:"eventType"`
	Data      json.RawMessage `json:"data"`
}

// receiveDeliveryEvents processes delivery events
func (s *Server) receiveDeliveryEvents(c *gin.Context) {
	var req DeliveryEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch req.EventType {
	case "CreateDeliveryNote":
		var cmd handlers.CreateDeliveryNoteCommand
		if err := json.Unmarshal(req.Data, &cmd); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		// If AggregateID is not provided, generate a new one
		if cmd.AggregateID == "" {
			cmd.AggregateID = uuid.New().String()
		}

		if err := s.deliveryHandler.HandleCreateDeliveryNote(ctx, cmd); err != nil {
			log.Error().Err(err).Msg("Failed to handle CreateDeliveryNote command")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	case "AddDeliveryNoteItem":
		var cmd handlers.AddDeliveryItemsCommand
		if err := json.Unmarshal(req.Data, &cmd); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := s.deliveryHandler.HandleAddDeliveryItems(ctx, cmd); err != nil {
			log.Error().Err(err).Msg("Failed to handle AddDeliveryItems command")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	case "RemoveDeliveryNoteItem":
		var cmd handlers.RemoveDeliveryItemCommand
		if err := json.Unmarshal(req.Data, &cmd); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := s.deliveryHandler.HandleRemoveDeliveryItem(ctx, cmd); err != nil {
			log.Error().Err(err).Msg("Failed to handle RemoveDeliveryItem command")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported event type"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "event processed successfully"})
}