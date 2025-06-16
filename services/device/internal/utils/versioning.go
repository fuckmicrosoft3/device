package utils

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"example.com/backstage/services/device/internal/models"
)

var (
	// semVerRegex matches a semantic version string (e.g., 1.2.3, 1.2.3-beta.1, 1.2.3+build.123)
	semVerRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
)

// ParseSemanticVersion parses a semantic version string into a SemanticVersion struct
func ParseSemanticVersion(version string) (*models.SemanticVersion, error) {
	matches := semVerRegex.FindStringSubmatch(version)
	if matches == nil {
		return nil, errors.New("invalid semantic version format")
	}

	major, err := strconv.ParseUint(matches[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse major version: %w", err)
	}

	minor, err := strconv.ParseUint(matches[2], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse minor version: %w", err)
	}

	patch, err := strconv.ParseUint(matches[3], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse patch version: %w", err)
	}

	semVer := &models.SemanticVersion{
		Major:      uint(major),
		Minor:      uint(minor),
		Patch:      uint(patch),
		PreRelease: matches[4],
		Build:      matches[5],
	}

	return semVer, nil
}

// FormatSemanticVersion formats a SemanticVersion struct into a semantic version string
func FormatSemanticVersion(version *models.SemanticVersion) string {
	result := fmt.Sprintf("%d.%d.%d", version.Major, version.Minor, version.Patch)
	
	if version.PreRelease != "" {
		result += "-" + version.PreRelease
	}
	
	if version.Build != "" {
		result += "+" + version.Build
	}
	
	return result
}

// CompareSemanticVersions compares two semantic versions
// Returns:
//   - -1 if v1 < v2
//   -  0 if v1 = v2
//   -  1 if v1 > v2
func CompareSemanticVersions(v1, v2 *models.SemanticVersion) int {
	// Compare major version
	if v1.Major < v2.Major {
		return -1
	}
	if v1.Major > v2.Major {
		return 1
	}

	// Compare minor version
	if v1.Minor < v2.Minor {
		return -1
	}
	if v1.Minor > v2.Minor {
		return 1
	}

	// Compare patch version
	if v1.Patch < v2.Patch {
		return -1
	}
	if v1.Patch > v2.Patch {
		return 1
	}

	// If we reach here, the core version components are equal
	// PreRelease versions are considered lower precedence than the same version without a PreRelease
	if v1.PreRelease == "" && v2.PreRelease != "" {
		return 1
	}
	if v1.PreRelease != "" && v2.PreRelease == "" {
		return -1
	}
	if v1.PreRelease != "" && v2.PreRelease != "" {
		// Compare pre-release identifiers one by one
		v1PreReleaseParts := strings.Split(v1.PreRelease, ".")
		v2PreReleaseParts := strings.Split(v2.PreRelease, ".")
		
		for i := 0; i < len(v1PreReleaseParts) && i < len(v2PreReleaseParts); i++ {
			// Numeric identifiers have lower precedence than non-numeric
			v1IsNum := isNumeric(v1PreReleaseParts[i])
			v2IsNum := isNumeric(v2PreReleaseParts[i])
			
			if v1IsNum && !v2IsNum {
				return -1
			}
			if !v1IsNum && v2IsNum {
				return 1
			}
			
			if v1IsNum && v2IsNum {
				v1Num, _ := strconv.Atoi(v1PreReleaseParts[i])
				v2Num, _ := strconv.Atoi(v2PreReleaseParts[i])
				if v1Num < v2Num {
					return -1
				}
				if v1Num > v2Num {
					return 1
				}
			} else {
				// Lexical comparison for non-numeric identifiers
				if v1PreReleaseParts[i] < v2PreReleaseParts[i] {
					return -1
				}
				if v1PreReleaseParts[i] > v2PreReleaseParts[i] {
					return 1
				}
			}
		}
		
		// If one pre-release has more identifiers, it has higher precedence
		if len(v1PreReleaseParts) < len(v2PreReleaseParts) {
			return -1
		}
		if len(v1PreReleaseParts) > len(v2PreReleaseParts) {
			return 1
		}
	}

	// Build metadata does not affect precedence
	return 0
}

// isNumeric checks if a string contains only numeric characters
func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// CompareVersionStrings compares two semantic version strings
// Returns:
//   - -1 if v1 < v2
//   -  0 if v1 = v2
//   -  1 if v1 > v2
//   - Error if either version string is invalid
func CompareVersionStrings(v1Str, v2Str string) (int, error) {
	v1, err := ParseSemanticVersion(v1Str)
	if err != nil {
		return 0, fmt.Errorf("invalid version 1: %w", err)
	}

	v2, err := ParseSemanticVersion(v2Str)
	if err != nil {
		return 0, fmt.Errorf("invalid version 2: %w", err)
	}

	return CompareSemanticVersions(v1, v2), nil
}

// IsValidVersionUpgrade checks if upgrading from oldVersion to newVersion is valid
// according to semantic versioning rules
func IsValidVersionUpgrade(oldVersionStr, newVersionStr string) (bool, error) {
	result, err := CompareVersionStrings(oldVersionStr, newVersionStr)
	if err != nil {
		return false, err
	}
	
	// New version should be greater than old version
	return result < 0, nil
}

// IsValidTestVersion checks if a test version is higher than its corresponding production version
func IsValidTestVersion(testVersionStr, prodVersionStr string) (bool, error) {
	if prodVersionStr == "" {
		// If there's no production version, any test version is valid
		return true, nil
	}
	
	result, err := CompareVersionStrings(testVersionStr, prodVersionStr)
	if err != nil {
		return false, err
	}
	
	// Test version should be greater than production version
	return result > 0, nil
}

// IncrementMajor increments the major version and resets minor and patch to 0
func IncrementMajor(version *models.SemanticVersion) *models.SemanticVersion {
	return &models.SemanticVersion{
		Major:      version.Major + 1,
		Minor:      0,
		Patch:      0,
		PreRelease: "",
		Build:      "",
	}
}

// IncrementMinor increments the minor version and resets patch to 0
func IncrementMinor(version *models.SemanticVersion) *models.SemanticVersion {
	return &models.SemanticVersion{
		Major:      version.Major,
		Minor:      version.Minor + 1,
		Patch:      0,
		PreRelease: "",
		Build:      "",
	}
}

// IncrementPatch increments the patch version
func IncrementPatch(version *models.SemanticVersion) *models.SemanticVersion {
	return &models.SemanticVersion{
		Major:      version.Major,
		Minor:      version.Minor,
		Patch:      version.Patch + 1,
		PreRelease: "",
		Build:      "",
	}
}

// AddPreRelease adds a pre-release tag to a version
func AddPreRelease(version *models.SemanticVersion, preRelease string) *models.SemanticVersion {
	return &models.SemanticVersion{
		Major:      version.Major,
		Minor:      version.Minor,
		Patch:      version.Patch,
		PreRelease: preRelease,
		Build:      version.Build,
	}
}

// AddBuild adds a build metadata tag to a version
func AddBuild(version *models.SemanticVersion, build string) *models.SemanticVersion {
	return &models.SemanticVersion{
		Major:      version.Major,
		Minor:      version.Minor,
		Patch:      version.Patch,
		PreRelease: version.PreRelease,
		Build:      build,
	}
}