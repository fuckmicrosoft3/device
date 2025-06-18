package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/rs/zerolog/log"

	"go.novek.io/canister/handlers"
)

// EventType definitions
const (
	CreateCanister        = "CreateCanister"
	UpdateCanister        = "UpdateCanister"
	CanisterExit          = "CanisterExit"
	CanisterEntry         = "CanisterEntry"
	CanisterDamage        = "CanisterDamage"
	CanisterRestoreDamage = "RestoreCanisterDamage"
	CanisterRestoreTamper = "RestoreCanisterTamper"
	CanisterCheck         = "CanisterCheck"
	CanisterOrgCheckIn    = "OrgCheckIn"
	CanisterOrgCheckOut   = "OrgCheckOut"
	DeliveryNoteCreated   = "CreateDeliveryNote"
	DeliveryItemAdded     = "AddDeliveryNoteItem"
	DeliveryItemRemoved   = "RemoveDeliveryNoteItem"
)

// AzureBusMessage is the common message structure
type AzureBusMessage struct {
	EventType string          `json:"eventType"`
	Data      json.RawMessage `json:"data"`
}

type MessageProcessor interface {
	ProcessMessage(ctx context.Context, message *azservicebus.ReceivedMessage) error
}

type Processor struct {
	canisterHandler *handlers.CanisterHandler
	deliveryHandler *handlers.DeliveryHandler
}

func NewProcessor(canisterHandler *handlers.CanisterHandler, deliveryHandler *handlers.DeliveryHandler) *Processor {
	return &Processor{
		canisterHandler: canisterHandler,
		deliveryHandler: deliveryHandler,
	}
}

func (p *Processor) ProcessMessage(ctx context.Context, message *azservicebus.ReceivedMessage) error {
	var msg AzureBusMessage
	if err := json.Unmarshal(message.Body, &msg); err != nil {
		return fmt.Errorf("error unmarshalling message: %w", err)
	}

	log.Info().Str("eventType", msg.EventType).Msg("Processing message")

	switch msg.EventType {
	// Canister commands
	case CreateCanister:
		var cmd handlers.CreateCanisterCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCreateCanister(ctx, cmd)

	case UpdateCanister:
		var cmd handlers.UpdateCanisterCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleUpdateCanister(ctx, cmd)

	case CanisterEntry:
		var cmd handlers.CanisterEntryCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterEntry(ctx, cmd)

	case CanisterExit:
		var cmd handlers.CanisterExitCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterExit(ctx, cmd)

	case CanisterCheck:
		var cmd handlers.CanisterCheckCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterCheck(ctx, cmd)

	case CanisterDamage:
		var cmd handlers.CanisterDamageCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterDamage(ctx, cmd)

	case CanisterOrgCheckIn:
		var cmd handlers.CanisterOrgCheckInCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterOrgCheckIn(ctx, cmd)

	case CanisterOrgCheckOut:
		var cmd handlers.CanisterOrgCheckOutCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterOrgCheckOut(ctx, cmd)

	case CanisterRestoreDamage:
		var cmd handlers.CanisterRestoreDamageCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterRestoreDamage(ctx, cmd)

	case CanisterRestoreTamper:
		var cmd handlers.CanisterRestoreTamperCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterRestoreTamper(ctx, cmd)

	// Delivery commands
	case DeliveryNoteCreated:
		var cmd handlers.CreateDeliveryNoteCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.deliveryHandler.HandleCreateDeliveryNote(ctx, cmd)

	case DeliveryItemAdded:
		var cmd handlers.AddDeliveryItemsCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.deliveryHandler.HandleAddDeliveryItems(ctx, cmd)

	case DeliveryItemRemoved:
		var cmd handlers.RemoveDeliveryItemCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			return err
		}
		return p.deliveryHandler.HandleRemoveDeliveryItem(ctx, cmd)

	default:
		// Check for custom event types in the message body
		// Special handling for events like 'check', 'can_entry', etc.
		return p.handleCustomEventType(ctx, message)
	}
}

func (p *Processor) handleCustomEventType(ctx context.Context, message *azservicebus.ReceivedMessage) error {
	// Parse the JSON content
	var parsedContent map[string]interface{}
	if err := json.Unmarshal(message.Body, &parsedContent); err != nil {
		return fmt.Errorf("failed to parse message: %w", err)
	}

	evValue, ok := parsedContent["ev"].(string)
	if !ok {
		return fmt.Errorf("ev field not found or not a string")
	}

	switch evValue {
	case "can_entry":
		var cmd handlers.CanisterEntryCommand
		if err := json.Unmarshal(message.Body, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterEntry(ctx, cmd)

	case "can_exit":
		var cmd handlers.CanisterExitCommand
		if err := json.Unmarshal(message.Body, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterExit(ctx, cmd)

	case "check":
		var cmd handlers.CanisterCheckCommand
		if err := json.Unmarshal(message.Body, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterCheck(ctx, cmd)

	case "can_refill":
		var cmd handlers.CanisterRefillSessionCommand
		if err := json.Unmarshal(message.Body, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterRefillSession(ctx, cmd)

	case "can_refiller_entry":
		var cmd handlers.CanisterRefillerEntryCommand
		if err := json.Unmarshal(message.Body, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterRefillerEntry(ctx, cmd)

	case "can_refiller_exit":
		var cmd handlers.CanisterRefillerExitCommand
		if err := json.Unmarshal(message.Body, &cmd); err != nil {
			return err
		}
		return p.canisterHandler.HandleCanisterRefillerExit(ctx, cmd)

	default:
		return fmt.Errorf("unsupported event type: %s", evValue)
	}
}
