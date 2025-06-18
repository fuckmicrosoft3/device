package projections

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v7"
	"gorm.io/gorm"

	"go.novek.io/canister/config"
	"go.novek.io/canister/domain"
	"go.novek.io/canister/models"
	"go.novek.io/canister/utils"
)

// Constants for index names
const (
	CanistersIndex         = "canisters"
	CanisterEventsIndex    = "canister-events"
	CanisterMovementsIndex = "canister-movements"
	CanisterRefillsIndex   = "canister-refills-sessions"
)

// CanisterProjector handles projections for canister events
type CanisterProjector struct {
	db            *gorm.DB
	elasticClient *elasticsearch.Client
	cfg           config.Config
}

// NewCanisterProjector creates a new canister projector
func NewCanisterProjector(db *gorm.DB, elasticClient *elasticsearch.Client, cfg config.Config) *CanisterProjector {
	return &CanisterProjector{
		db:            db,
		elasticClient: elasticClient,
		cfg:           cfg,
	}
}

// Project projects an event
func (p *CanisterProjector) Project(ctx context.Context, event domain.Event) error {
	switch event.Type {
	case domain.CanisterCreated:
		return p.projectCanisterCreated(ctx, event)
	case domain.CanisterUpdated:
		return p.projectCanisterUpdated(ctx, event)
	case domain.CanisterEntry:
		return p.projectCanisterEntry(ctx, event)
	case domain.CanisterExit:
		return p.projectCanisterExit(ctx, event)
	case domain.CanisterCheck:
		return p.projectCanisterCheck(ctx, event)
	case domain.CanisterDamage:
		return p.projectCanisterDamage(ctx, event)
	case domain.CanisterOrgCheckIn:
		return p.projectCanisterOrgCheckIn(ctx, event)
	case domain.CanisterOrgCheckOut:
		return p.projectCanisterOrgCheckOut(ctx, event)
	case domain.CanisterRestoreDamage:
		return p.projectCanisterRestoreDamage(ctx, event)
	case domain.CanisterRestoreTamper:
		return p.projectCanisterRestoreTamper(ctx, event)
	case domain.CanisterRefillerEntry:
		return p.projectCanisterRefillerEntry(ctx, event)
	case domain.CanisterRefillerExit:
		return p.projectCanisterRefillerExit(ctx, event)
	case domain.CanisterRefillSession:
		return p.projectCanisterRefillSession(ctx, event)
	default:
		return nil
	}
}

// projectCanisterCreated handles the canister created event
func (p *CanisterProjector) projectCanisterCreated(ctx context.Context, event domain.Event) error {
	// Parse event data
	var data domain.CanisterCreatedEvent
	if err := json.Unmarshal(event.Data.([]byte), &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	// Create a volume default value
	defaultVolume := 20.0
	defaultTamperState := "NO_TAMPER"

	// Create canister in database
	canister := models.Canister{
		CanisterID:    data.CanisterID,
		AggregateID:   event.AggregateID,
		Version:       int32(event.Version),
		Tag:           data.Tag,
		Mcu:           data.MCU,
		Model:         data.Model,
		Name:          data.Name,
		Status:        data.Status,
		Attributes:    data.Attributes,
		CurrentVolume: &defaultVolume,
		TamperState:   &defaultTamperState,
		CreatedAt:     event.Timestamp,
		UpdatedAt:     event.Timestamp,
	}

	if err := p.db.WithContext(ctx).Create(&canister).Error; err != nil {
		return fmt.Errorf("failed to create canister in database: %w", err)
	}

	// Index in Elasticsearch
	canisterDoc, err := json.Marshal(canister)
	if err != nil {
		return fmt.Errorf("failed to marshal canister: %w", err)
	}

	// Index canister in Elasticsearch
	index := FormatIndex(CanistersIndex, p.cfg)
	res, err := p.elasticClient.Index(
		index,
		bytes.NewReader(canisterDoc),
		p.elasticClient.Index.WithDocumentID(data.MCU),
		p.elasticClient.Index.WithRefresh("true"),
		p.elasticClient.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to index canister in Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to index canister in Elasticsearch: %s", res.String())
	}

	// Index event in Elasticsearch
	eventDoc, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	index = FormatIndex(CanisterEventsIndex, p.cfg)
	res, err = p.elasticClient.Index(
		index,
		bytes.NewReader(eventDoc),
		p.elasticClient.Index.WithDocumentID(event.ID),
		p.elasticClient.Index.WithRefresh("true"),
		p.elasticClient.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to index event in Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to index event in Elasticsearch: %s", res.String())
	}

	return nil
}

// projectCanisterUpdated handles the canister updated event
func (p *CanisterProjector) projectCanisterUpdated(ctx context.Context, event domain.Event) error {
	// Parse event data
	var data domain.CanisterUpdatedEvent
	if err := json.Unmarshal(event.Data.([]byte), &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	// Update canister in database
	updateFields := map[string]interface{}{
		"version":    event.Version,
		"tag":        data.Tag,
		"mcu":        data.MCU,
		"model":      data.Model,
		"name":       data.Name,
		"status":     data.Status,
		"attributes": data.Attributes,
		"updated_at": event.Timestamp,
	}

	if err := p.db.WithContext(ctx).Model(&models.Canister{}).
		Where("aggregate_id = ?", event.AggregateID).
		Updates(updateFields).Error; err != nil {
		return fmt.Errorf("failed to update canister in database: %w", err)
	}

	// Update in Elasticsearch
	updateDoc := map[string]interface{}{
		"doc": updateFields,
	}

	jsonBody, err := json.Marshal(updateDoc)
	if err != nil {
		return fmt.Errorf("failed to marshal update doc: %w", err)
	}

	index := FormatIndex(CanistersIndex, p.cfg)
	res, err := p.elasticClient.Update(
		index,
		data.MCU,
		bytes.NewReader(jsonBody),
		p.elasticClient.Update.WithRefresh("true"),
		p.elasticClient.Update.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to update canister in Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to update canister in Elasticsearch: %s", res.String())
	}

	// Index event in Elasticsearch
	eventDoc, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	index = FormatIndex(CanisterEventsIndex, p.cfg)
	res, err = p.elasticClient.Index(
		index,
		bytes.NewReader(eventDoc),
		p.elasticClient.Index.WithDocumentID(event.ID),
		p.elasticClient.Index.WithRefresh("true"),
		p.elasticClient.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to index event in Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to index event in Elasticsearch: %s", res.String())
	}

	return nil
}

// projectCanisterCheck handles the canister check event
func (p *CanisterProjector) projectCanisterCheck(ctx context.Context, event domain.Event) error {
	// Parse event data
	var data domain.CanisterCheckEvent
	if err := json.Unmarshal(event.Data.([]byte), &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	// Parse the payload to extract temperature and volume data
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(data.Payload), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Extract temperature and volume
	temperature := utils.GetFloat64Value(payload, "temp_obj")
	volume := utils.GetFloat64Value(payload, "volume")
	tamperState := utils.ConvertTamperState(utils.GetIntValue(payload, "tamp_state"))
	tamperSources := utils.ConvertTamperSources(utils.GetIntValue(payload, "tamp_srcs"))
	status := "ReadyForUse"
	if tamperState != "NO_TAMPER" {
		status = "Damaged"
	}

	// Update canister in database
	updateFields := map[string]interface{}{
		"current_temperature": temperature,
		"current_volume":      volume,
		"tamper_state":        tamperState,
		"tamper_sources":      tamperSources,
		"status":              status,
		"updated_at":          event.Timestamp,
	}

	if err := p.db.WithContext(ctx).Model(&models.Canister{}).
		Where("aggregate_id = ?", event.AggregateID).
		Updates(updateFields).Error; err != nil {
		return fmt.Errorf("failed to update canister in database: %w", err)
	}

	// Update in Elasticsearch
	updateDoc := map[string]interface{}{
		"doc": updateFields,
	}

	jsonBody, err := json.Marshal(updateDoc)
	if err != nil {
		return fmt.Errorf("failed to marshal update doc: %w", err)
	}

	// Get the MCU from database
	var canister models.Canister
	if err := p.db.WithContext(ctx).
		Where("aggregate_id = ?", event.AggregateID).
		First(&canister).Error; err != nil {
		return fmt.Errorf("failed to get canister from database: %w", err)
	}

	index := FormatIndex(CanistersIndex, p.cfg)
	res, err := p.elasticClient.Update(
		index,
		canister.Mcu,
		bytes.NewReader(jsonBody),
		p.elasticClient.Update.WithRefresh("true"),
		p.elasticClient.Update.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to update canister in Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to update canister in Elasticsearch: %s", res.String())
	}

	// Index event in Elasticsearch
	eventDoc, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	index = FormatIndex(CanisterEventsIndex, p.cfg)
	res, err = p.elasticClient.Index(
		index,
		bytes.NewReader(eventDoc),
		p.elasticClient.Index.WithDocumentID(event.ID),
		p.elasticClient.Index.WithRefresh("true"),
		p.elasticClient.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to index event in Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to index event in Elasticsearch: %s", res.String())
	}

	return nil
}

// projectCanisterEntry handles the canister entry event
func (p *CanisterProjector) projectCanisterEntry(ctx context.Context, event domain.Event) error {
	// Implementation for canister entry event
	// This would create movement entries and update canister location
	return nil
}

// projectCanisterExit handles the canister exit event
func (p *CanisterProjector) projectCanisterExit(ctx context.Context, event domain.Event) error {
	// Implementation for canister exit event
	return nil
}

// projectCanisterDamage handles the canister damage event
func (p *CanisterProjector) projectCanisterDamage(ctx context.Context, event domain.Event) error {
	// Implementation for canister damage event
	return nil
}

// Other projection methods...
func (p *CanisterProjector) projectCanisterOrgCheckIn(ctx context.Context, event domain.Event) error {
	return nil
}

func (p *CanisterProjector) projectCanisterOrgCheckOut(ctx context.Context, event domain.Event) error {
	return nil
}

func (p *CanisterProjector) projectCanisterRestoreDamage(ctx context.Context, event domain.Event) error {
	return nil
}

func (p *CanisterProjector) projectCanisterRestoreTamper(ctx context.Context, event domain.Event) error {
	return nil
}

func (p *CanisterProjector) projectCanisterRefillerEntry(ctx context.Context, event domain.Event) error {
	return nil
}

func (p *CanisterProjector) projectCanisterRefillerExit(ctx context.Context, event domain.Event) error {
	return nil
}

func (p *CanisterProjector) projectCanisterRefillSession(ctx context.Context, event domain.Event) error {
	// Implementation for refill session, which might update volume and create refill history
	return nil
}
