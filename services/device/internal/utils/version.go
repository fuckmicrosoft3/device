// services/device/internal/utils/version.go
package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ValidateVersion validates semantic version format (e.g., 1.2.3).
func ValidateVersion(version string) error {
	// Simple semantic version validation
	pattern := `^v?(\d+)\.(\d+)\.(\d+)(-[a-zA-Z0-9\-\.]+)?(\+[a-zA-Z0-9\-\.]+)?$`
	matched, err := regexp.MatchString(pattern, version)
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("invalid version format: %s", version)
	}
	return nil
}

// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
func CompareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Split versions
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Compare major, minor, patch
	for i := range 3 {
		var num1, num2 int

		if i < len(parts1) {
			num1, _ = strconv.Atoi(strings.Split(parts1[i], "-")[0])
		}
		if i < len(parts2) {
			num2, _ = strconv.Atoi(strings.Split(parts2[i], "-")[0])
		}

		if num1 < num2 {
			return -1
		}
		if num1 > num2 {
			return 1
		}
	}

	return 0
}
