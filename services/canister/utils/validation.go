package utils

import (
	"fmt"
	"regexp"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// ValidateStruct validates a struct using validation tags
func ValidateStruct(s interface{}) error {
	if err := validate.Struct(s); err != nil {
		return err
	}
	return nil
}

// IsValidUUID checks if a string is a valid UUID
func IsValidUUID(uuid string) bool {
	r := regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$")
	return r.MatchString(uuid)
}

// IsValidMCU checks if a string is a valid MCU identifier
func IsValidMCU(mcu string) bool {
	// Implement MCU validation logic based on your requirements
	return len(mcu) > 0
}

// ValidateCanisterID validates a canister ID
func ValidateCanisterID(id string) error {
	if id == "" {
		return fmt.Errorf("canister ID cannot be empty")
	}
	return nil
}

// ValidateAggregateID validates an aggregate ID
func ValidateAggregateID(id string) error {
	if id == "" {
		return fmt.Errorf("aggregate ID cannot be empty")
	}
	return nil
}

// RegisterCustomValidations registers custom validation functions
func RegisterCustomValidations() {
	validate.RegisterValidation("canister_id", func(fl validator.FieldLevel) bool {
		return ValidateCanisterID(fl.Field().String()) == nil
	})

	validate.RegisterValidation("aggregate_id", func(fl validator.FieldLevel) bool {
		return ValidateAggregateID(fl.Field().String()) == nil
	})

	validate.RegisterValidation("mcu", func(fl validator.FieldLevel) bool {
		return IsValidMCU(fl.Field().String())
	})
}
