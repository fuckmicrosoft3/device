package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Canister represents a canister in the database
type Canister struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	CanisterID         string         `gorm:"uniqueIndex" json:"canister_id"`
	AggregateID        string         `gorm:"uniqueIndex" json:"aggregate_id"`
	Version            int32          `json:"version"`
	Tag                string         `json:"tag"`
	Mcu                string         `gorm:"uniqueIndex" json:"mcu"`
	Model              string         `json:"model"`
	Name               string         `json:"name"`
	OrganizationID     uuid.UUID      `gorm:"index" json:"organization_id"`
	Status             string         `json:"status"`
	Attributes         []byte         `json:"attributes"`
	Lastmovementid     *uuid.UUID     `json:"lastmovementid"`
	InBound            bool           `json:"in_bound"`
	OutBound           bool           `json:"out_bound"`
	CurrentTemperature *float64       `json:"current_temperature"`
	CurrentVolume      *float64       `json:"current_volume"`
	TamperState        *string        `json:"tamper_state"`
	TamperSources      *string        `json:"tamper_sources"`
	LastTenEvents      []byte         `json:"last_ten_events"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// CanisterMovement represents a canister movement in the database
type CanisterMovement struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	MovementID  uuid.UUID      `gorm:"uniqueIndex" json:"movement_id"`
	CanisterID  *string        `gorm:"index" json:"canister_id"`
	FromPointID *uuid.UUID     `json:"from_point_id"`
	ToPointID   *uuid.UUID     `json:"to_point_id"`
	LeftTime    time.Time      `json:"left_time"`
	ArrivalTime time.Time      `json:"arrival_time"`
	Status      string         `json:"status"`
	Attributes  []byte         `json:"attributes"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// CanisterMovementPoint represents a movement point in the database
type CanisterMovementPoint struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	PointID        uuid.UUID      `gorm:"uniqueIndex" json:"point_id"`
	Type           string         `json:"type"`
	MachineID      *uuid.UUID     `json:"machine_id"`
	OrganisationID *uuid.UUID     `json:"organisation_id"`
	TruckID        *uuid.UUID     `json:"truck_id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// Machine represents a machine in the database
type Machine struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	MachineID  uuid.UUID      `gorm:"uniqueIndex" json:"machine_id"`
	DeviceMcu  string         `gorm:"uniqueIndex" json:"device_mcu"`
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	Attributes []byte         `json:"attributes"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// Organization represents an organization in the database
type Organization struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	OrgID       uuid.UUID      `gorm:"uniqueIndex" json:"org_id"`
	OrgName     string         `json:"org_name"`
	ParentOrgID *uuid.UUID     `json:"parent_org_id"`
	Attributes  []byte         `json:"attributes"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// CompleteMovement represents a complete movement in the database
type CompleteMovement struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	MovementID  uuid.UUID      `gorm:"uniqueIndex" json:"movement_id"`
	CanisterID  string         `gorm:"index" json:"canister_id"`
	FromPointID *uuid.UUID     `json:"from_point_id"`
	ToPointID   *uuid.UUID     `json:"to_point_id"`
	FromType    *string        `json:"from_type"`
	ToType      *string        `json:"to_type"`
	TypeIDFrom  *uuid.UUID     `json:"type_id_from"`
	TypeIDTo    *uuid.UUID     `json:"type_id_to"`
	LeftTime    time.Time      `json:"left_time"`
	ArrivalTime time.Time      `json:"arrival_time"`
	Attributes  []byte         `json:"attributes"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}
