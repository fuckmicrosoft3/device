package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/google/uuid"

	"go.novek.io/canister/domain"
	"go.novek.io/canister/eventstore"
)

// Command structs
type CreateCanisterCommand struct {
	AggregateID    string            `json:"aggregate_id"`
	CanisterID     string            `json:"id"`
	Tag            string            `json:"tag"`
	MCU            string            `json:"mcu"`
	Model          string            `json:"model_name"`
	Name           string            `json:"model_id"`
	Status         string            `json:"status"`
	OrganisationID string            `json:"organisation_id"`
	Attributes     map[string]string `json:"attributes"`
 

type UpdateCanisterCommand struct {
	CanisterID     string            `json:"canister_id"`
	Version        int32             `json:"version"`
	AggregateID    string            `json:"aggregate_id"`
	Tag            string            `json:"tag"`
	MCU            string            `json:"mcu"`
	Model          string            `json:"model"`
	Name           string            `json:"name"`
	OrganisationID string            `json:"organisationid"`
	Status         string            `json:"status"`
	Attributes     map[string]string `json:"attributes"`
}

type CanisterEntryCommand struct {
	AggregateID   string    `json:"aggregate_id"`
	CanisterID    string    `json:"canister_id"`
	Version       int32     `json:"version"`
	ExchangerUUID string    `json:"exchanger_uuid"`
	Ev            string    `json:"ev"`
	Serial        string    `json:"serial"`
	MCU           string    `json:"mcu"`
	DeviceID      string    `json:"device_id"`
	Source        string    `json:"source"`
	SourceID      string    `json:"source_id"`
	SourceTopic   string    `json:"source_topic"`
	Payload       string    `json:"payload"`
	Duplicate     bool      `json:"duplicate"`
	Time          time.Time `json:"time"`
}

type CanisterExitCommand struct {
	AggregateID   string    `json:"aggregate_id"`
	CanisterID    string    `json:"canister_id"`
	Version       int32     `json:"version"`
	ExchangerUUID string    `json:"exchanger_uuid"`
	Ev            string    `json:"ev"`
	Serial        string    `json:"serial"`
	MCU           string    `json:"mcu"`
	DeviceID      string    `json:"device_id"`
	Source        string    `json:"source"`
	SourceID      string    `json:"source_id"`
	SourceTopic   string    `json:"source_topic"`
	Payload       string    `json:"payload"`
	Duplicate     bool      `json:"duplicate"`
	Time          time.Time `json:"time"`
}

type CanisterCheckCommand struct {
	CanisterID    string    `json:"canister_id"`
	Version       int32     `json:"version"`
	AggregateID   string    `json:"aggregate_id"`
	ExchangerUUID string    `json:"exchanger_uuid"`
	Ev            string    `json:"ev"`
	Serial        string    `json:"serial"`
	MCU           string    `json:"mcu"`
	DeviceID      string    `json:"device_id"`
	Source        string    `json:"source"`
	SourceID      string    `json:"source_id"`
	SourceTopic   string    `json:"source_topic"`
	Payload       string    `json:"payload"`
	Duplicate     bool      `json:"duplicate"`
	Time          time.Time `json:"time"`
	TracingHeader string    `json:"tracing_header"`
}

type CanisterDamageCommand struct {
	CanisterID    string `json:"canister_id"`
	Version       int32  `json:"version"`
	AggregateID   string `json:"aggregate_id"`
	Time          string `json:"time"`
	ReportedBy    string `json:"reported_by"`
	ReportedByOrg string `json:"reported_by_org"`
	Reason        string `json:"reason"`
}

type CanisterOrgCheckInCommand struct {
	CanisterID  string `json:"canister_id"`
	Version     int32  `json:"version"`
	AggregateID string `json:"aggregate_id"`
	ID          string `json:"id"`
	MCU         string `json:"mcu"`
	Time        string `json:"time"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	UserOrgID   string `json:"user_org_id"`
	UserOrgName string `json:"user_org_name"`
}

type CanisterOrgCheckOutCommand struct {
	CanisterID  string `json:"canister_id"`
	Version     int32  `json:"version"`
	AggregateID string `json:"aggregate_id"`
	ID          string `json:"id"`
	MCU         string `json:"mcu"`
	Time        string `json:"time"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	UserOrgID   string `json:"user_org_id"`
	UserOrgName string `json:"user_org_name"`
}

type CanisterRestoreDamageCommand struct {
	CanisterID    string `json:"canister_id"`
	Version       int32  `json:"version"`
	AggregateID   string `json:"aggregate_id"`
	Time          string `json:"time"`
	RestoredBy    string `json:"restored_by"`
	RestoredByOrg string `json:"restored_by_org"`
}

type CanisterRestoreTamperCommand struct {
	CanisterID    string `json:"canister_id"`
	Version       int32  `json:"version"`
	AggregateID   string `json:"aggregate_id"`
	Time          string `json:"time"`
	RestoredBy    string `json:"restored_by"`
	RestoredByOrg string `json:"restored_by_org"`
}

type CanisterRefillerEntryCommand struct {
	CanisterID    string `json:"canister_id"`
	Version       int32  `json:"version"`
	AggregateID   string `json:"aggregate_id"`
	ExchangerUUID string `json:"exchanger_uuid"`
	Ev            string `json:"ev"`
	Serial        string `json:"serial"`
	MCU           string `json:"mcu"`
	DeviceID      string `json:"device_id"`
	Source        string `json:"source"`
	SourceID      string `json:"source_id"`
	SourceTopic   string `json:"source_topic"`
	Payload       string `json:"payload"`
	RefillerID    string `json:"refiller_id"`
	SessionID     string `json:"session_id"`
	Status        int32  `json:"status"`
	Time          int32  `json:"time"`
}

type CanisterRefillerExitCommand struct {
	CanisterID    string `json:"canister_id"`
	Version       int32  `json:"version"`
	AggregateID   string `json:"aggregate_id"`
	ExchangerUUID string `json:"exchanger_uuid"`
	Ev            string `json:"ev"`
	Serial        string `json:"serial"`
	MCU           string `json:"mcu"`
	DeviceID      string `json:"device_id"`
	Source        string `json:"source"`
	SourceID      string `json:"source_id"`
	SourceTopic   string `json:"source_topic"`
	Payload       string `json:"payload"`
	RefillerID    string `json:"refiller_id"`
	SessionID     string `json:"session_id"`
	Status        int32  `json:"status"`
	Time          int32  `json:"time"`
}

type CanisterRefillSessionCommand struct {
	CanisterID    string  `json:"canister_id"`
	Version       int32   `json:"version"`
	AggregateID   string  `json:"aggregate_id"`
	RefillerID    string  `json:"refiller_id"`
	Serial        string  `json:"serial"`
	Ev            string  `json:"ev"`
	DeviceID      string  `json:"device_id"`
	Source        string  `json:"source"`
	SourceID      string  `json:"source_id"`
	SourceTopic   string  `json:"source_topic"`
	ExchangerUUID string  `json:"exchanger_uuid"`
	StartTime     int64   `json:"time"`
	CurrentTime   int64   `json:"current_time"`
	TargetVolume  float64 `json:"target_volume"`
	ActualVolume  float64 `json:"actual_volume"`
	Status        int32   `json:"status"`
	SessionID     string  `json:"session_id"`
	Payload       string  `json:"payload"`
}

// CanisterHandler handles all canister-related commands
type CanisterHandler struct {
	store eventstore.EventStore
}

// NewCanisterHandler creates a new canister handler
func NewCanisterHandler(store eventstore.EventStore) *CanisterHandler {
	return &CanisterHandler{store: store}
}

// HandleCreateCanister creates a new canister
func (h *CanisterHandler) HandleCreateCanister(ctx context.Context, cmd CreateCanisterCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CreateCanister command")

	// Check if canister already exists
	exists, err := h.store.Exists(ctx, cmd.AggregateID)
	if err != nil {
		return fmt.Errorf("failed to check if canister exists: %w", err)
	}

	if exists {
		return fmt.Errorf("canister already exists with ID %s", cmd.AggregateID)
	}

	// Create a new aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)

	// Convert attributes to JSON
	attrBytes, err := json.Marshal(cmd.Attributes)
	if err != nil {
		return fmt.Errorf("failed to marshal attributes: %w", err)
	}

	// Create the event
	event := domain.CanisterCreatedEvent{
		CanisterID:     cmd.CanisterID,
		Tag:            cmd.Tag,
		MCU:            cmd.MCU,
		Model:          cmd.Model,
		Name:           cmd.Name,
		Status:         cmd.Status,
		OrganisationID: cmd.OrganisationID,
		Attributes:     attrBytes,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleUpdateCanister updates a canister
func (h *CanisterHandler) HandleUpdateCanister(ctx context.Context, cmd UpdateCanisterCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling UpdateCanister command")

	// Load the aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Convert attributes to JSON
	attrBytes, err := json.Marshal(cmd.Attributes)
	if err != nil {
		return fmt.Errorf("failed to marshal attributes: %w", err)
	}

	// Create the event
	event := domain.CanisterUpdatedEvent{
		CanisterID:     cmd.CanisterID,
		Tag:            cmd.Tag,
		MCU:            cmd.MCU,
		Model:          cmd.Model,
		Name:           cmd.Name,
		Status:         cmd.Status,
		OrganisationID: cmd.OrganisationID,
		Attributes:     attrBytes,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleCanisterEntry handles a canister entry
func (h *CanisterHandler) HandleCanisterEntry(ctx context.Context, cmd CanisterEntryCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CanisterEntry command")

	// Load the aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.CanisterEntryEvent{
		CanisterID:    cmd.CanisterID,
		ExchangerUUID: cmd.ExchangerUUID,
		Ev:            cmd.Ev,
		Serial:        cmd.Serial,
		MCU:           cmd.MCU,
		DeviceID:      cmd.DeviceID,
		Source:        cmd.Source,
		SourceID:      cmd.SourceID,
		SourceTopic:   cmd.SourceTopic,
		Payload:       cmd.Payload,
		Duplicate:     cmd.Duplicate,
		Time:          cmd.Time,
		MovementID:    uuid.New().String(),
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleCanisterExit handles a canister exit
func (h *CanisterHandler) HandleCanisterExit(ctx context.Context, cmd CanisterExitCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CanisterExit command")

	// Load the aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.CanisterExitEvent{
		CanisterID:    cmd.CanisterID,
		ExchangerUUID: cmd.ExchangerUUID,
		Ev:            cmd.Ev,
		Serial:        cmd.Serial,
		MCU:           cmd.MCU,
		DeviceID:      cmd.DeviceID,
		Source:        cmd.Source,
		SourceID:      cmd.SourceID,
		SourceTopic:   cmd.SourceTopic,
		Payload:       cmd.Payload,
		Duplicate:     cmd.Duplicate,
		Time:          cmd.Time,
		MovementID:    uuid.New().String(),
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleCanisterCheck handles a canister check
func (h *CanisterHandler) HandleCanisterCheck(ctx context.Context, cmd CanisterCheckCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CanisterCheck command")

	// Load the aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.CanisterCheckEvent{
		CanisterID:    cmd.CanisterID,
		ExchangerUUID: cmd.ExchangerUUID,
		Ev:            cmd.Ev,
		Serial:        cmd.Serial,
		MCU:           cmd.MCU,
		DeviceID:      cmd.DeviceID,
		Source:        cmd.Source,
		SourceID:      cmd.SourceID,
		SourceTopic:   cmd.SourceTopic,
		Payload:       cmd.Payload,
		Duplicate:     cmd.Duplicate,
		Time:          cmd.Time,
		TracingHeader: cmd.TracingHeader,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleCanisterDamage handles a canister damage
func (h *CanisterHandler) HandleCanisterDamage(ctx context.Context, cmd CanisterDamageCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CanisterDamage command")

	// Load the aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.CanisterDamageEvent{
		CanisterID:    cmd.CanisterID,
		Time:          cmd.Time,
		ReportedBy:    cmd.ReportedBy,
		ReportedByOrg: cmd.ReportedByOrg,
		Reason:        cmd.Reason,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleCanisterOrgCheckIn handles a canister organization check-in
func (h *CanisterHandler) HandleCanisterOrgCheckIn(ctx context.Context, cmd CanisterOrgCheckInCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CanisterOrgCheckIn command")

	// Load the aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.CanisterOrgCheckInEvent{
		CanisterID:  cmd.CanisterID,
		MCU:         cmd.MCU,
		Time:        cmd.Time,
		UserID:      cmd.UserID,
		UserName:    cmd.UserName,
		UserOrgID:   cmd.UserOrgID,
		UserOrgName: cmd.UserOrgName,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleCanisterOrgCheckOut handles a canister organization check-out
func (h *CanisterHandler) HandleCanisterOrgCheckOut(ctx context.Context, cmd CanisterOrgCheckOutCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CanisterOrgCheckOut command")

	// Load the aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.CanisterOrgCheckOutEvent{
		CanisterID:  cmd.CanisterID,
		MCU:         cmd.MCU,
		Time:        cmd.Time,
		UserID:      cmd.UserID,
		UserName:    cmd.UserName,
		UserOrgID:   cmd.UserOrgID,
		UserOrgName: cmd.UserOrgName,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleCanisterRestoreDamage handles a canister damage restoration
func (h *CanisterHandler) HandleCanisterRestoreDamage(ctx context.Context, cmd CanisterRestoreDamageCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CanisterRestoreDamage command")

	// Load the aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.CanisterRestoreDamageEvent{
		CanisterID:    cmd.CanisterID,
		Time:          cmd.Time,
		RestoredBy:    cmd.RestoredBy,
		RestoredByOrg: cmd.RestoredByOrg,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleCanisterRestoreTamper handles a canister tamper restoration
func (h *CanisterHandler) HandleCanisterRestoreTamper(ctx context.Context, cmd CanisterRestoreTamperCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CanisterRestoreTamper command")

	// Load the aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.CanisterRestoreTamperEvent{
		CanisterID:    cmd.CanisterID,
		Time:          cmd.Time,
		RestoredBy:    cmd.RestoredBy,
		RestoredByOrg: cmd.RestoredByOrg,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleCanisterRefillerEntry handles a refiller entry
func (h *CanisterHandler) HandleCanisterRefillerEntry(ctx context.Context, cmd CanisterRefillerEntryCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CanisterRefillerEntry command")

	// Load the aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.CanisterRefillerEntryEvent{
		CanisterID:    cmd.CanisterID,
		ExchangerUUID: cmd.ExchangerUUID,
		Serial:        cmd.Serial,
		MCU:           cmd.MCU,
		DeviceID:      cmd.DeviceID,
		Source:        cmd.Source,
		SourceID:      cmd.SourceID,
		SourceTopic:   cmd.SourceTopic,
		Payload:       cmd.Payload,
		RefillerID:    cmd.RefillerID,
		SessionID:     cmd.SessionID,
		Status:        cmd.Status,
		Time:          cmd.Time,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleCanisterRefillerExit handles a refiller exit
func (h *CanisterHandler) HandleCanisterRefillerExit(ctx context.Context, cmd CanisterRefillerExitCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CanisterRefillerExit command")

	// Load the aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.CanisterRefillerExitEvent{
		CanisterID:    cmd.CanisterID,
		ExchangerUUID: cmd.ExchangerUUID,
		Serial:        cmd.Serial,
		MCU:           cmd.MCU,
		DeviceID:      cmd.DeviceID,
		Source:        cmd.Source,
		SourceID:      cmd.SourceID,
		SourceTopic:   cmd.SourceTopic,
		Payload:       cmd.Payload,
		RefillerID:    cmd.RefillerID,
		SessionID:     cmd.SessionID,
		Status:        cmd.Status,
		Time:          cmd.Time,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}

// HandleCanisterRefillSession handles a refill session
func (h *CanisterHandler) HandleCanisterRefillSession(ctx context.Context, cmd CanisterRefillSessionCommand) error {
	log.Info().Str("aggregateID", cmd.AggregateID).Msg("Handling CanisterRefillSession command")

	// Load the aggregate
	aggregate := domain.NewCanisterAggregate(cmd.AggregateID)
	if err := h.store.Load(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to load aggregate: %w", err)
	}

	// Create the event
	event := domain.CanisterRefillSessionEvent{
		CanisterID:    cmd.CanisterID,
		ExchangerUUID: cmd.ExchangerUUID,
		Serial:        cmd.Serial,
		Ev:            cmd.Ev,
		RefillerID:    cmd.RefillerID,
		DeviceID:      cmd.DeviceID,
		Source:        cmd.Source,
		SourceID:      cmd.SourceID,
		SourceTopic:   cmd.SourceTopic,
		Payload:       cmd.Payload,
		StartTime:     cmd.StartTime,
		CurrentTime:   cmd.CurrentTime,
		TargetVolume:  cmd.TargetVolume,
		ActualVolume:  cmd.ActualVolume,
		Status:        cmd.Status,
		SessionID:     cmd.SessionID,
	}

	// Apply the event
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	// Save the aggregate
	if err := h.store.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate: %w", err)
	}

	return nil
}