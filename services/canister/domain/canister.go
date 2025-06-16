package domain

// CanisterState represents the state of a canister
type CanisterState struct {
	CanisterID         string
	AggregateID        string
	Version            int
	Tag                string
	MCU                string
	Model              string
	Name               string
	OrganisationID     string
	Status             string
	Attributes         []byte
	LastMovementID     string
	CurrentTemperature float64
	CurrentVolume      float64
	TamperState        string
	TamperSources      []string
}

// CanisterAggregate is the aggregate for a canister
type CanisterAggregate struct {
	*AggregateBase
	State CanisterState
}

// NewCanisterAggregate creates a new canister aggregate
func NewCanisterAggregate(id string) *CanisterAggregate {
	aggregate := &CanisterAggregate{
		State: CanisterState{
			AggregateID: id,
		},
	}
	
	base := NewAggregateBase("canister", aggregate.applyEvent)
	base.SetID(id)
	aggregate.AggregateBase = base
	
	return aggregate
}

// applyEvent applies an event to the canister aggregate
func (a *CanisterAggregate) applyEvent(event any) error {
	switch e := event.(type) {
	case CanisterCreatedEvent:
		a.State.CanisterID = e.CanisterID
		a.State.Tag = e.Tag
		a.State.MCU = e.MCU
		a.State.Model = e.Model
		a.State.Name = e.Name
		a.State.Status = e.Status
		a.State.OrganisationID = e.OrganisationID
		a.State.Attributes = e.Attributes
		// Default values
		a.State.CurrentVolume = 20.0
		a.State.TamperState = "NO_TAMPER"
		
	case CanisterUpdatedEvent:
		a.State.Tag = e.Tag
		a.State.MCU = e.MCU
		a.State.Model = e.Model
		a.State.Name = e.Name
		a.State.Status = e.Status
		a.State.OrganisationID = e.OrganisationID
		a.State.Attributes = e.Attributes
		
	case CanisterEntryEvent:
		a.State.LastMovementID = e.MovementID
		
	case CanisterExitEvent:
		a.State.LastMovementID = e.MovementID
		
	case CanisterCheckEvent:
		// Parse payload to extract temperature, volume, and tamper information
		
	case CanisterDamageEvent:
		a.State.Status = "Damaged"
		
	case CanisterRestoreDamageEvent:
		a.State.Status = "ReadyForUse"
		
	case CanisterRestoreTamperEvent:
		a.State.TamperState = "NO_TAMPER"
		a.State.TamperSources = nil
		
	case CanisterRefillSessionEvent:
		a.State.CurrentVolume = e.ActualVolume
	}
	
	return nil
}