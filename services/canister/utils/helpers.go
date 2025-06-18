package utils

import (
	"encoding/json"
	"fmt"
)

// GetFloat64Value safely extracts a float64 value from a map
func GetFloat64Value(data map[string]interface{}, key string) float64 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			if f, err := parseFloat(v); err == nil {
				return f
			}
		}
	}
	return 0
}

// GetIntValue safely extracts an int value from a map
func GetIntValue(data map[string]interface{}, key string) int {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case float32:
			return int(v)
		case string:
			if i, err := parseInt(v); err == nil {
				return i
			}
		}
	}
	return 0
}

// GetStringValue safely extracts a string value from a map
func GetStringValue(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case int, int64, float64, float32, bool:
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

// ConvertTamperState converts numeric tamper state to string
func ConvertTamperState(state int) string {
	switch state {
	case 0:
		return "NO_TAMPER"
	case 1:
		return "TAMPERED"
	default:
		return "UNKNOWN_TAMPER_STATE"
	}
}

// ConvertTamperSources converts numeric tamper sources to string
func ConvertTamperSources(source int) string {
	switch source {
	case 1:
		return "EMI"
	case 2:
		return "TAMPER_SWITCH"
	case 0:
		return "NO_TAMPER"
	default:
		return "UNKNOWN_TAMPER_SOURCE"
	}
}

// GetRefillSessionStatus converts numeric status to string
func GetRefillSessionStatus(status int) string {
	switch status {
	case 1:
		return "complete"
	case 0:
		return "in-progress"
	default:
		return "session error"
	}
}

// ParseAttributes parses JSON attributes
func ParseAttributes(attributes []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if len(attributes) == 0 {
		return result, nil
	}

	if err := json.Unmarshal(attributes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
	}

	return result, nil
}

// ExtractOrganizationIDs extracts organization IDs from attributes
func ExtractOrganizationIDs(attributes []byte) ([]string, error) {
	type Organization struct {
		OrgID       string `json:"orgId"`
		OrgName     string `json:"orgName"`
		ParentOrgID string `json:"parentOrgId"`
	}

	type Organizations struct {
		Organizations []Organization `json:"organisations"`
	}

	var orgs Organizations
	if err := json.Unmarshal(attributes, &orgs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal organizations: %w", err)
	}

	var orgIDs []string
	for _, org := range orgs.Organizations {
		orgIDs = append(orgIDs, org.OrgID)
	}

	return orgIDs, nil
}

// parseFloat parses a string to float64
func parseFloat(s string) (float64, error) {
	var f float64
	err := json.Unmarshal([]byte(s), &f)
	return f, err
}

// parseInt parses a string to int
func parseInt(s string) (int, error) {
	var i int
	err := json.Unmarshal([]byte(s), &i)
	return i, err
}
