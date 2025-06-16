package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"example.com/backstage/services/truck/internal/model"
	"example.com/backstage/services/truck/internal/repository"
)

// RecordEventRequest defines the request to record an operation event
type RecordEventRequest struct {
	Event            string                 `json:"ev" validate:"required"`
	SessionID        string                 `json:"u" validate:"required"`
	OperationID      string                 `json:"op"`
	OperationGroupID string                 `json:"opg"`
	DeviceMCU        string                 `json:"device_mcu"`
	ExtraParams      map[string]interface{} `json:"params"`
}

// OperationEventService defines the interface for operation event service
type OperationEventService interface {
	RecordEvent(ctx context.Context, req *RecordEventRequest) error
}

// operationEventService implements OperationEventService
type operationEventService struct {
	operationService      OperationService
	operationGroupService OperationGroupService
	eventRepo             repository.OperationEventRepository
	deviceRepo            repository.DeviceRepository
}

// NewOperationEventService creates a new operation event service
func NewOperationEventService(
	operationService OperationService,
	operationGroupService OperationGroupService,
	eventRepo repository.OperationEventRepository,
	deviceRepo repository.DeviceRepository,
) OperationEventService {
	return &operationEventService{
		operationService:      operationService,
		operationGroupService: operationGroupService,
		eventRepo:             eventRepo,
		deviceRepo:            deviceRepo,
	}
}

// RecordEvent records an operation event
func (s *operationEventService) RecordEvent(ctx context.Context, req *RecordEventRequest) error {
	var operation *model.Operation
	var operationGroup *model.OperationGroup
	var operationSession *model.OperationSession
	var err error

	// Get operation if ID is provided
	if req.OperationID != "" {
		operation, err = s.operationService.GetByID(ctx, req.OperationID)
		if err != nil {
			return fmt.Errorf("failed to get operation: %w", err)
		}
	}

	// Get operation group if ID is provided
	if req.OperationGroupID != "" {
		operationGroup, err = s.operationGroupService.GetByID(ctx, req.OperationGroupID)
		if err != nil {
			return fmt.Errorf("failed to get operation group: %w", err)
		}
	}

	// Get operation session if session ID is provided
	if req.SessionID != "" {
		operationSession, err = s.operationService.GetOperationSessionByID(ctx, req.SessionID)
		if err != nil || operationSession == nil {
			// Only fail if this is not a start event
			if model.EventTypeFromString(req.Event) != model.OperationStartEvent {
				return fmt.Errorf("failed to get operation session: %w", err)
			}
		}
	}

	// Get device by MCU
	device, err := s.deviceRepo.FindByMCU(ctx, req.DeviceMCU)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}

	// Need at least an operation or an operation group
	if operation == nil && operationGroup == nil {
		return errors.New("no operation or operation group found")
	}

	// Handle different event types
	eventType := model.EventTypeFromString(req.Event)
	switch eventType {
	case model.OperationAcknowledgementEvent:
		if operation != nil {
			operation.Status = model.AcknowledgedOperationStatus
		}
		if operationGroup != nil {
			operationGroup.Status = model.AcknowledgedOperationStatus
		}

	case model.OperationStartEvent:
		if operationSession == nil {
			// Create a new session
			start := time.Now()
			operationSession = &model.OperationSession{
				Base: model.Base{
					UUID: req.SessionID,
				},
				OperationID:      operation.UUID,
				OperationGroupID: operation.OperationGroupID,
				StartedAt:        &start,
			}
		}

		// Parse target volumes from extra params
		if targetVolumeOut, ok := req.ExtraParams["t_v_out"].(float64); ok {
			operationSession.TargetVolumeOut = &targetVolumeOut
		}
		if targetVolumeIn, ok := req.ExtraParams["t_v_in"].(float64); ok {
			operationSession.TargetVolumeIn = &targetVolumeIn
		}

		// Update operation status
		if operation != nil {
			operation.Status = model.InProgressOperationStatus
		}
		if operationGroup != nil {
			operationGroup.Status = model.InProgressOperationStatus
		}

	case model.OperationStatusEvent:
		// Update volume out
		if volumeOut, ok := req.ExtraParams["v_out"].(float64); ok {
			if operationSession.SessionVolumeOut != nil {
				volumeOut = math.Max(*operationSession.SessionVolumeOut, volumeOut)
			}
			operationSession.SessionVolumeOut = &volumeOut
		}

		// Update volume in
		if volumeIn, ok := req.ExtraParams["v_in"].(float64); ok {
			if operationSession.SessionVolumeIn != nil {
				volumeIn = math.Max(*operationSession.SessionVolumeIn, volumeIn)
			}
			operationSession.SessionVolumeIn = &volumeIn
		}

	case model.OperationCompleteEvent:
		// Update session volumes
		if sessionVolumeOut, ok := req.ExtraParams["s_v_out"].(float64); ok {
			operationSession.SessionVolumeOut = &sessionVolumeOut
		}
		if sessionVolumeIn, ok := req.ExtraParams["s_v_in"].(float64); ok {
			operationSession.SessionVolumeIn = &sessionVolumeIn
		}

		// Mark session as complete
		completedAt := time.Now()
		operationSession.CompletedAt = &completedAt
		operationSession.Complete = true
		operationSession.Error = false

		// Update operation status
		if operation != nil {
			operation.Status = model.CompleteOperationStatus
		}

		// Check if all operations in the group are complete
		groupID := ""
		if operation != nil {
			groupID = operation.OperationGroupID
		}
		if operationGroup != nil {
			groupID = operationGroup.UUID
		}

		if groupID != "" {
			// Find all active operations in the group
			activeOps, err := s.operationService.FindActiveOperationsByOperationGroup(ctx, groupID)
			if err != nil {
				logrus.WithError(err).Error("Failed to get active operations by group")
			} else {
				// If there are no pending operations, or if there is only one and it's the current one
				if len(activeOps) == 0 || (len(activeOps) == 1 && activeOps[0].UUID == operation.UUID) {
					if operationGroup == nil {
						operationGroup, err = s.operationGroupService.GetByID(ctx, groupID)
						if err != nil {
							logrus.WithError(err).Error("Failed to get operation group")
						} else {
							operationGroup.Status = model.CompleteOperationStatus
						}
					} else {
						operationGroup.Status = model.CompleteOperationStatus
					}
				}
			}
		}

	case model.OperationErrorEvent:
		// Mark session as error
		completedAt := time.Now()
		operationSession.CompletedAt = &completedAt
		operationSession.Complete = false
		operationSession.Error = true

		// Update session volumes
		if sessionVolumeOut, ok := req.ExtraParams["s_v_out"].(float64); ok {
			if operationSession.SessionVolumeOut != nil {
				sessionVolumeOut = math.Max(*operationSession.SessionVolumeOut, sessionVolumeOut)
			}
			operationSession.SessionVolumeOut = &sessionVolumeOut
		}
		if sessionVolumeIn, ok := req.ExtraParams["s_v_in"].(float64); ok {
			if operationSession.SessionVolumeIn != nil {
				sessionVolumeIn = math.Max(*operationSession.SessionVolumeIn, sessionVolumeIn)
			}
			operationSession.SessionVolumeIn = &sessionVolumeIn
		}

		// Update operation status
		if operation != nil {
			operation.Status = model.ErrorOperationStatus
		}

	case model.OperationCancelEvent:
		// Mark session as cancelled
		completedAt := time.Now()
		operationSession.CompletedAt = &completedAt
		operationSession.Complete = false
		operationSession.Error = true

		// Update operation and group status
		if operation != nil {
			operation.Status = model.CancelledOperationStatus
		}
		if operationGroup != nil {
			operationGroup.Status = model.CancelledOperationStatus
		}
	}

	// Update operation, operation group, and operation session
	if operation != nil {
		_, err = s.operationService.Update(ctx, operation)
		if err != nil {
			return fmt.Errorf("failed to update operation: %w", err)
		}
	}

	if operationGroup != nil {
		_, err = s.operationGroupService.Update(ctx, operationGroup)
		if err != nil {
			return fmt.Errorf("failed to update operation group: %w", err)
		}
	}

	if operationSession != nil {
		_, err = s.operationService.CreateUpdateOperationSession(ctx, operationSession)
		if err != nil {
			return fmt.Errorf("failed to update operation session: %w", err)
		}
	}

	// Create and store the event
	event := &model.OperationEvent{
		Base: model.Base{
			UUID: uuid.New().String(),
		},
		DeviceID:     device.UUID,
		DeviceType:   device.Type,
		EventType:    eventType,
	}

	if operation != nil {
		opID := operation.UUID
		event.OperationID = &opID
	}

	if operationGroup != nil {
		event.OperationGroupID = operationGroup.UUID
	} else if operation != nil && operation.OperationGroupID != "" {
		event.OperationGroupID = operation.OperationGroupID
	}

	if operationSession != nil {
		sessionID := operationSession.UUID
		event.OperationSessionID = &sessionID
	}

	// Store extra params as JSON
	detailsJSON, err := json.Marshal(req.ExtraParams)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal event details")
	} else {
		event.Details = detailsJSON
	}

	// Store the event
	if err := s.eventRepo.Create(ctx, event); err != nil {
		// Log but don't fail the request
		logrus.WithError(err).Error("Failed to store operation event")
	}

	return nil
}