package domain

// DeliveryState represents the state of a delivery note
type DeliveryState struct {
	ID             string
	OrganizationID string
	DeliveryItems  []DeliveryItem
}

// DeliveryItem represents an item in a delivery note
type DeliveryItem struct {
	ID             string `json:"id"`
	CanisterID     string `json:"canister_id"`
	DeliveryNoteID string `json:"delivery_note_id"`
	Delivered      bool   `json:"delivered"`
}

// DeliveryAggregate is the aggregate for a delivery note
type DeliveryNoteAggregate struct {
	*AggregateBase
	State DeliveryState
}

// NewDeliveryAggregate creates a new delivery aggregate
func NewDeliveryNoteAggregate(id string) *DeliveryNoteAggregate {
	aggregate := &DeliveryNoteAggregate{
		State: DeliveryState{
			ID: id,
		},
	}
	
	base := NewAggregateBase("delivery", aggregate.applyEvent)
	base.SetID(id)
	aggregate.AggregateBase = base
	
	return aggregate
}

// applyEvent applies an event to the delivery aggregate
func (a *DeliveryNoteAggregate) applyEvent(event interface{}) error {
	switch e := event.(type) {
	case DeliveryNoteCreatedEvent:
		a.State.ID = e.ID
		a.State.OrganizationID = e.OrganizationID
		
	case DeliveryItemsAddedEvent:
		a.State.DeliveryItems = append(a.State.DeliveryItems, e.DeliveryItems...)
		
	case DeliveryItemRemovedEvent:
		// Remove item with the specified ID
		for i, item := range a.State.DeliveryItems {
			if item.ID == e.ID {
				a.State.DeliveryItems = append(a.State.DeliveryItems[:i], a.State.DeliveryItems[i+1:]...)
				break
			}
		}
	}
	
	return nil
}