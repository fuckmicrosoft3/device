package utils

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON marshals data to JSON
func MarshalJSON(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// UnmarshalJSON unmarshals JSON data
func UnmarshalJSON(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}

// PrettyPrint returns a pretty-printed JSON string
func PrettyPrint(data interface{}) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(bytes), nil
}

// FromJSON converts a JSON string to a map
func FromJSON(jsonStr string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return result, nil
}

// ToJSON converts a map to a JSON string
func ToJSON(data map[string]interface{}) (string, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(bytes), nil
}

// DataToJSON converts any data to a JSON string
func DataToJSON(data interface{}) (string, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(bytes), nil
}