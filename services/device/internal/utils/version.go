package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Regex to parse semantic versioning (e.g., 1.2.3-beta.1).
var versionRegex = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-(.+))?$`)

// ValidateVersion checks if a string conforms to the expected version format.
func ValidateVersion(version string) error {
	if !versionRegex.MatchString(version) {
		return fmt.Errorf("invalid version format: %s. Expected format: X.Y.Z or X.Y.Z-prerelease", version)
	}
	return nil
}

// ParseVersion breaks a version string into its components.
func ParseVersion(version string) (major, minor, patch int, prerelease string, err error) {
	matches := versionRegex.FindStringSubmatch(version)
	if matches == nil {
		return 0, 0, 0, "", fmt.Errorf("invalid version format: %s", version)
	}

	major, _ = strconv.Atoi(matches[1])
	minor, _ = strconv.Atoi(matches[2])
	patch, _ = strconv.Atoi(matches[3])
	if len(matches) > 4 {
		prerelease = matches[4]
	}

	return major, minor, patch, prerelease, nil
}

// CompareVersions compares two semantic version strings.
// Returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	 1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	maj1, min1, pat1, pre1, _ := ParseVersion(v1)
	maj2, min2, pat2, pre2, _ := ParseVersion(v2)

	// Compare major version
	if maj1 < maj2 {
		return -1
	}
	if maj1 > maj2 {
		return 1
	}

	// Compare minor version
	if min1 < min2 {
		return -1
	}
	if min1 > min2 {
		return 1
	}

	// Compare patch version
	if pat1 < pat2 {
		return -1
	}
	if pat1 > pat2 {
		return 1
	}

	// Compare prerelease tags
	if pre1 == "" && pre2 != "" {
		return 1 // v1 is a release, v2 is a prerelease
	}
	if pre1 != "" && pre2 == "" {
		return -1 // v1 is a prerelease, v2 is a release
	}

	// Standard string comparison for prerelease tags
	return strings.Compare(pre1, pre2)
}
