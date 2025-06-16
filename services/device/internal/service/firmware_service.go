package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/repository"
	
	"github.com/sirupsen/logrus"
)

// FirmwareService defines the firmware-specific operations
type FirmwareService interface {
	// Firmware management
	UploadFirmware(ctx context.Context, file io.Reader, filename string, releaseType models.ReleaseType, version string, isTest bool, notes string) (*models.FirmwareReleaseExtended, error)
	ValidateFirmware(ctx context.Context, releaseID uint) (*models.FirmwareReleaseValidation, error)
	SignFirmware(ctx context.Context, releaseID uint, privateKeyPEM string) error
	PromoteTestToProduction(ctx context.Context, testReleaseID uint) (*models.FirmwareReleaseExtended, error)
	
	// Version management
	ParseVersion(version string) (*models.SemanticVersion, error)
	CompareVersions(v1, v2 string) (int, error)
	
	// Retrieval methods
	GetFirmwareByVersion(ctx context.Context, version string, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error)
	ListFirmwareVersions(ctx context.Context, releaseType models.ReleaseType) ([]*models.FirmwareReleaseExtended, error)
	GetLatestFirmware(ctx context.Context, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error)
	GetFirmwareFile(ctx context.Context, releaseID uint) (string, io.ReadCloser, error)
	
	// Manifest management
	GenerateManifest(ctx context.Context, releaseType models.ReleaseType) (*models.FirmwareManifest, error)
	SignManifest(ctx context.Context, manifestID uint, privateKeyPEM string) error
}

// firmwareService implements FirmwareService
type firmwareService struct {
	repo           repository.Repository
	firmwareRepo   repository.FirmwareRepository
	log            *logrus.Logger
	storagePath    string
	signatureKeys  map[string]*ecdsa.PrivateKey
	defaultKeyID   string
}

// NewFirmwareService creates a new firmware service
func NewFirmwareService(
	repo repository.Repository,
	firmwareRepo repository.FirmwareRepository,
	log *logrus.Logger,
	storagePath string,
) (FirmwareService, error) {
	// Create firmware storage directory if it doesn't exist
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create firmware storage directory: %w", err)
	}
	
	// Initialize the signature keys map
	signatureKeys := make(map[string]*ecdsa.PrivateKey)
	
	// Look for existing keys or generate a new one
	keyPath := filepath.Join(storagePath, "keys")
	if err := os.MkdirAll(keyPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create keys directory: %w", err)
	}
	
	// Try to load existing key
	defaultKeyID := "default"
	keyFile := filepath.Join(keyPath, "default.pem")
	
	var defaultKey *ecdsa.PrivateKey
	
	// If key file exists, load it
	if _, err := os.Stat(keyFile); err == nil {
		keyData, err := ioutil.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read key file: %w", err)
		}
		
		block, _ := pem.Decode(keyData)
		if block == nil {
			return nil, errors.New("failed to decode PEM block containing private key")
		}
		
		privateKey, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		
		defaultKey = privateKey
	} else {
		// Generate a new key
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate private key: %w", err)
		}
		
		// Encode to PEM format
		keyBytes, err := x509.MarshalECPrivateKey(privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal private key: %w", err)
		}
		
		pemBlock := &pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: keyBytes,
		}
		
		// Save to file
		if err := ioutil.WriteFile(keyFile, pem.EncodeToMemory(pemBlock), 0600); err != nil {
			return nil, fmt.Errorf("failed to save private key: %w", err)
		}
		
		defaultKey = privateKey
	}
	
	// Add default key to map
	signatureKeys[defaultKeyID] = defaultKey
	
	return &firmwareService{
		repo:          repo,
		firmwareRepo:  firmwareRepo,
		log:           log,
		storagePath:   storagePath,
		signatureKeys: signatureKeys,
		defaultKeyID:  defaultKeyID,
	}, nil
}

// UploadFirmware handles the firmware upload process
func (s *firmwareService) UploadFirmware(
	ctx context.Context, 
	file io.Reader, 
	filename string, 
	releaseType models.ReleaseType, 
	version string, 
	isTest bool, 
	notes string,
) (*models.FirmwareReleaseExtended, error) {
	// Parse and validate version
	semVer, err := s.ParseVersion(version)
	if err != nil {
		return nil, fmt.Errorf("invalid version format: %w", err)
	}
	
	// Create storage directory for this release
	releaseDir := filepath.Join(s.storagePath, string(releaseType), version)
	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create release directory: %w", err)
	}
	
	// Generate a unique filename
	timestamp := time.Now().Format("20060102150405")
	filePath := filepath.Join(releaseDir, fmt.Sprintf("%s-%s.bin", version, timestamp))
	
	// Create the file
	outFile, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create firmware file: %w", err)
	}
	defer outFile.Close()
	
	// Hash the file as we write it
	hash := sha256.New()
	writer := io.MultiWriter(outFile, hash)
	
	// Copy the file and get its size
	size, err := io.Copy(writer, file)
	if err != nil {
		os.Remove(filePath) // Clean up on error
		return nil, fmt.Errorf("failed to write firmware file: %w", err)
	}
	
	// Get the file hash
	fileHash := hex.EncodeToString(hash.Sum(nil))
	
	// Create firmware release record
	firmwareRelease := &models.FirmwareReleaseExtended{
		FirmwareRelease: models.FirmwareRelease{
			FilePath:    filePath,
			ReleaseType: releaseType,
			Version:     version,
			Size:        uint(size),
			Valid:       false, // Will be validated later
			IsTest:      isTest,
			FileHash:    fileHash,
		},
		MajorVersion:      semVer.Major,
		MinorVersion:      semVer.Minor,
		PatchVersion:      semVer.Patch,
		PreReleaseVersion: semVer.PreRelease,
		BuildMetadata:     semVer.Build,
		ReleaseNotes:      notes,
	}
	
	// Save to database
	if err := s.firmwareRepo.CreateFirmwareRelease(ctx, firmwareRelease); err != nil {
		os.Remove(filePath) // Clean up on error
		return nil, fmt.Errorf("failed to save firmware release: %w", err)
	}
	
	// Start validation in background
	go func() {
		bgCtx := context.Background()
		_, err := s.ValidateFirmware(bgCtx, firmwareRelease.ID)
		if err != nil {
			s.log.WithError(err).Errorf("Failed to validate firmware %d", firmwareRelease.ID)
		}
	}()
	
	return firmwareRelease, nil
}

// ValidateFirmware validates a firmware release
func (s *firmwareService) ValidateFirmware(ctx context.Context, releaseID uint) (*models.FirmwareReleaseValidation, error) {
	// Get the firmware release
	release, err := s.firmwareRepo.GetFirmwareReleaseByID(ctx, releaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get firmware release: %w", err)
	}
	
	validation := &models.FirmwareReleaseValidation{
		FirmwareReleaseID: releaseID,
		ValidationStatus:  "in_progress",
		ValidatedAt:       time.Now(),
		ValidatedBy:       "system",
	}
	
	// Validate file existence
	if _, err := os.Stat(release.FilePath); os.IsNotExist(err) {
		validation.ValidationStatus = "failed"
		validation.ValidationErrors = "Firmware file not found"
		validation.HashValid = false
		
		// Save validation results
		if err := s.firmwareRepo.CreateFirmwareValidation(ctx, validation); err != nil {
			s.log.WithError(err).Error("Failed to save firmware validation results")
		}
		
		return validation, errors.New("firmware file not found")
	}
	
	// Open the file
	file, err := os.Open(release.FilePath)
	if err != nil {
		validation.ValidationStatus = "failed"
		validation.ValidationErrors = fmt.Sprintf("Failed to open firmware file: %v", err)
		
		// Save validation results
		if err := s.firmwareRepo.CreateFirmwareValidation(ctx, validation); err != nil {
			s.log.WithError(err).Error("Failed to save firmware validation results")
		}
		
		return validation, fmt.Errorf("failed to open firmware file: %w", err)
	}
	defer file.Close()
	
	// Validate file hash
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		validation.ValidationStatus = "failed"
		validation.ValidationErrors = fmt.Sprintf("Failed to read firmware file: %v", err)
		
		// Save validation results
		if err := s.firmwareRepo.CreateFirmwareValidation(ctx, validation); err != nil {
			s.log.WithError(err).Error("Failed to save firmware validation results")
		}
		
		return validation, fmt.Errorf("failed to read firmware file: %w", err)
	}
	
	calculatedHash := hex.EncodeToString(hash.Sum(nil))
	validation.HashValid = calculatedHash == release.FileHash
	
	// Validate file size
	fileInfo, err := file.Stat()
	if err != nil {
		validation.ValidationStatus = "failed"
		validation.ValidationErrors = fmt.Sprintf("Failed to get file info: %v", err)
		
		// Save validation results
		if err := s.firmwareRepo.CreateFirmwareValidation(ctx, validation); err != nil {
			s.log.WithError(err).Error("Failed to save firmware validation results")
		}
		
		return validation, fmt.Errorf("failed to get file info: %w", err)
	}
	
	validation.SizeValid = uint(fileInfo.Size()) == release.Size
	
	// Validate version format
	_, err = s.ParseVersion(release.Version)
	validation.VersionValid = err == nil
	
	// Validate signature if present
	if release.Signature != "" {
		validation.SignatureValid = s.verifySignature(release)
	} else {
		validation.SignatureValid = false
	}
	
	// Check if all validations passed
	if validation.HashValid && validation.SizeValid && validation.VersionValid {
		validation.ValidationStatus = "passed"
	} else {
		validation.ValidationStatus = "failed"
		var errors []string
		
		if !validation.HashValid {
			errors = append(errors, "Hash validation failed")
		}
		if !validation.SizeValid {
			errors = append(errors, "Size validation failed")
		}
		if !validation.VersionValid {
			errors = append(errors, "Version validation failed")
		}
		if !validation.SignatureValid && release.Signature != "" {
			errors = append(errors, "Signature validation failed")
		}
		
		validation.ValidationErrors = strings.Join(errors, "; ")
	}
	
	// Save validation results
	if err := s.firmwareRepo.CreateFirmwareValidation(ctx, validation); err != nil {
		s.log.WithError(err).Error("Failed to save firmware validation results")
		return validation, fmt.Errorf("failed to save validation results: %w", err)
	}
	
	// Update firmware release
	release.Valid = validation.ValidationStatus == "passed"
	if err := s.firmwareRepo.UpdateFirmwareRelease(ctx, release); err != nil {
		s.log.WithError(err).Error("Failed to update firmware release")
		return validation, fmt.Errorf("failed to update firmware release: %w", err)
	}
	
	return validation, nil
}

// SignFirmware signs a firmware release
func (s *firmwareService) SignFirmware(ctx context.Context, releaseID uint, privateKeyPEM string) error {
	// Get the firmware release
	release, err := s.firmwareRepo.GetFirmwareReleaseByID(ctx, releaseID)
	if err != nil {
		return fmt.Errorf("failed to get firmware release: %w", err)
	}
	
	// Parse the private key if provided, otherwise use default
	var privateKey *ecdsa.PrivateKey
	
	if privateKeyPEM != "" {
		block, _ := pem.Decode([]byte(privateKeyPEM))
		if block == nil {
			return errors.New("failed to decode PEM block containing private key")
		}
		
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		
		privateKey = key
	} else {
		// Use default key
		var ok bool
		privateKey, ok = s.signatureKeys[s.defaultKeyID]
		if !ok {
			return errors.New("default signing key not found")
		}
	}
	
	// Read the firmware file
	fileData, err := s.readFileWithValidation(release.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read firmware file: %w", err)
	}
	
	// Hash the file
	hash := sha256.Sum256(fileData)
	
	// Sign the hash
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	if err != nil {
		return fmt.Errorf("failed to sign firmware: %w", err)
	}
	
	// Format the signature
	signature := fmt.Sprintf("%x,%x", r, s)
	
	// Update the firmware release
	release.Signature = signature
	release.SignatureAlgorithm = "ecdsa-secp256r1"
	now := time.Now()
	release.SignedAt = &now
	release.SignedBy = "system"
	release.CertificateID = s.defaultKeyID
	
	// Save to database
	if err := s.firmwareRepo.UpdateFirmwareRelease(ctx, release); err != nil {
		return fmt.Errorf("failed to update firmware release: %w", err)
	}
	
	return nil
}

// PromoteTestToProduction promotes a test release to production
func (s *firmwareService) PromoteTestToProduction(ctx context.Context, testReleaseID uint) (*models.FirmwareReleaseExtended, error) {
	// Get the test release
	testRelease, err := s.firmwareRepo.GetFirmwareReleaseByID(ctx, testReleaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get test release: %w", err)
	}
	
	// Validate that this is a test release
	if !testRelease.IsTest {
		return nil, errors.New("firmware release is not a test release")
	}
	
	// Validate that the test passed
	if !testRelease.TestPassed {
		return nil, errors.New("test release has not passed testing")
	}
	
	// Create a production release based on this test release
	productionRelease := &models.FirmwareReleaseExtended{
		FirmwareRelease: models.FirmwareRelease{
			FilePath:    testRelease.FilePath,
			ReleaseType: models.ReleaseTypeProduction,
			Version:     testRelease.Version,
			Size:        testRelease.Size,
			Valid:       testRelease.Valid,
			IsTest:      false,
			FileHash:    testRelease.FileHash,
			TestReleaseID: &testRelease.ID,
		},
		MajorVersion:      testRelease.MajorVersion,
		MinorVersion:      testRelease.MinorVersion,
		PatchVersion:      testRelease.PatchVersion,
		PreReleaseVersion: "", // Remove pre-release tag for production
		BuildMetadata:     testRelease.BuildMetadata,
		Signature:         testRelease.Signature,
		SignatureAlgorithm: testRelease.SignatureAlgorithm,
		SignedAt:          testRelease.SignedAt,
		SignedBy:          testRelease.SignedBy,
		CertificateID:     testRelease.CertificateID,
		ReleaseNotes:      testRelease.ReleaseNotes,
	}
	
	// Save to database
	if err := s.firmwareRepo.CreateFirmwareRelease(ctx, productionRelease); err != nil {
		return nil, fmt.Errorf("failed to create production release: %w", err)
	}
	
	return productionRelease, nil
}

// ParseVersion parses a semantic version string
func (s *firmwareService) ParseVersion(version string) (*models.SemanticVersion, error) {
	semVer := &models.SemanticVersion{}
	
	// Split into version and metadata
	parts := strings.SplitN(version, "+", 2)
	versionPart := parts[0]
	
	// Extract build metadata if present
	if len(parts) > 1 {
		semVer.Build = parts[1]
	}
	
	// Split into version and pre-release
	parts = strings.SplitN(versionPart, "-", 2)
	corePart := parts[0]
	
	// Extract pre-release if present
	if len(parts) > 1 {
		semVer.PreRelease = parts[1]
	}
	
	// Parse core version (X.Y.Z)
	versionParts := strings.Split(corePart, ".")
	if len(versionParts) != 3 {
		return nil, fmt.Errorf("invalid semantic version format: %s", version)
	}
	
	// Parse major version
	major, err := strconv.ParseUint(versionParts[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", versionParts[0])
	}
	semVer.Major = uint(major)
	
	// Parse minor version
	minor, err := strconv.ParseUint(versionParts[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", versionParts[1])
	}
	semVer.Minor = uint(minor)
	
	// Parse patch version
	patch, err := strconv.ParseUint(versionParts[2], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %s", versionParts[2])
	}
	semVer.Patch = uint(patch)
	
	return semVer, nil
}

// CompareVersions compares two semantic versions
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func (s *firmwareService) CompareVersions(v1, v2 string) (int, error) {
	// Parse versions
	semVer1, err := s.ParseVersion(v1)
	if err != nil {
		return 0, fmt.Errorf("failed to parse version 1: %w", err)
	}
	
	semVer2, err := s.ParseVersion(v2)
	if err != nil {
		return 0, fmt.Errorf("failed to parse version 2: %w", err)
	}
	
	// Compare using the helper method
	return s.compareSemanticVersions(semVer1, semVer2)
}

// GetFirmwareByVersion gets a firmware release by version
func (s *firmwareService) GetFirmwareByVersion(ctx context.Context, version string, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error) {
	return s.firmwareRepo.GetFirmwareReleaseByVersion(ctx, version, releaseType)
}

// ListFirmwareVersions lists available firmware versions
func (s *firmwareService) ListFirmwareVersions(ctx context.Context, releaseType models.ReleaseType) ([]*models.FirmwareReleaseExtended, error) {
	return s.firmwareRepo.ListFirmwareReleases(ctx, releaseType)
}

// GetLatestFirmware gets the latest firmware release
func (s *firmwareService) GetLatestFirmware(ctx context.Context, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error) {
	return s.firmwareRepo.GetLatestFirmwareRelease(ctx, releaseType)
}

// GetFirmwareFile gets the firmware file
func (s *firmwareService) GetFirmwareFile(ctx context.Context, releaseID uint) (string, io.ReadCloser, error) {
	// Get the firmware release
	release, err := s.firmwareRepo.GetFirmwareReleaseByID(ctx, releaseID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get firmware release %d: %w", releaseID, err)
	}
	
	// Check if file exists
	if _, err := os.Stat(release.FilePath); os.IsNotExist(err) {
		return "", nil, fmt.Errorf("firmware file not found at path %s: %w", release.FilePath, err)
	}
	
	// Open the file
	file, err := os.Open(release.FilePath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to open firmware file %s: %w", release.FilePath, err)
	}
	
	return filepath.Base(release.FilePath), file, nil
}

// GenerateManifest generates a firmware manifest
func (s *firmwareService) GenerateManifest(ctx context.Context, releaseType models.ReleaseType) (*models.FirmwareManifest, error) {
	// Get all valid releases of this type
	releases, err := s.firmwareRepo.ListValidFirmwareReleases(ctx, releaseType, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list firmware releases: %w", err)
	}
	
	// Find the minimum and recommended versions
	var minVersion, recVersion string
	var minVersionParsed, recVersionParsed *models.SemanticVersion
	
	for _, release := range releases {
		semVer, err := s.ParseVersion(release.Version)
		if err != nil {
			s.log.WithError(err).Warnf("Failed to parse version %s", release.Version)
			continue
		}
		
		// Skip pre-release versions for min/rec determination
		if semVer.PreRelease != "" {
			continue
		}
		
		// Initialize min version
		if minVersion == "" {
			minVersion = release.Version
			minVersionParsed = semVer
		}
		
		// Initialize recommended version
		if recVersion == "" {
			recVersion = release.Version
			recVersionParsed = semVer
		}
		
		// Check if this is a lower min version
		comp, err := s.compareSemanticVersions(semVer, minVersionParsed)
		if err == nil && comp < 0 {
			minVersion = release.Version
			minVersionParsed = semVer
		}
		
		// Check if this is a higher recommended version
		comp, err = s.compareSemanticVersions(semVer, recVersionParsed)
		if err == nil && comp > 0 {
			recVersion = release.Version
			recVersionParsed = semVer
		}
	}
	
	// Create the manifest
	manifest := &models.FirmwareManifest{
		ManifestVersion:    "1.0",
		GeneratedAt:        time.Now(),
		MinimumVersion:     minVersion,
		RecommendedVersion: recVersion,
	}
	
	// Save to database
	if err := s.firmwareRepo.CreateFirmwareManifest(ctx, manifest, releases); err != nil {
		return nil, fmt.Errorf("failed to create firmware manifest: %w", err)
	}
	
	return manifest, nil
}

// SignManifest signs a firmware manifest
func (s *firmwareService) SignManifest(ctx context.Context, manifestID uint, privateKeyPEM string) error {
	// Get the manifest
	manifest, err := s.firmwareRepo.GetFirmwareManifest(ctx, manifestID)
	if err != nil {
		return fmt.Errorf("failed to get firmware manifest: %w", err)
	}
	
	// Parse the private key if provided, otherwise use default
	var privateKey *ecdsa.PrivateKey
	
	if privateKeyPEM != "" {
		block, _ := pem.Decode([]byte(privateKeyPEM))
		if block == nil {
			return errors.New("failed to decode PEM block containing private key")
		}
		
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		
		privateKey = key
	} else {
		// Use default key
		var ok bool
		privateKey, ok = s.signatureKeys[s.defaultKeyID]
		if !ok {
			return errors.New("default signing key not found")
		}
	}
	
	// Create a string representation of the manifest for signing
	manifestStr := fmt.Sprintf(
		"manifest:%s|min:%s|rec:%s|gen:%s",
		manifest.ManifestVersion,
		manifest.MinimumVersion,
		manifest.RecommendedVersion,
		manifest.GeneratedAt.Format(time.RFC3339),
	)
	
	// Hash the manifest
	hash := sha256.Sum256([]byte(manifestStr))
	
	// Sign the hash
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	if err != nil {
		return fmt.Errorf("failed to sign manifest: %w", err)
	}
	
	// Format the signature
	signature := fmt.Sprintf("%x,%x", r, s)
	
	// Update the manifest
	manifest.Signature = signature
	manifest.SignatureAlgorithm = "ecdsa-secp256r1"
	
	// Save to database
	if err := s.firmwareRepo.UpdateFirmwareManifest(ctx, manifest); err != nil {
		return fmt.Errorf("failed to update firmware manifest: %w", err)
	}
	
	return nil
}

// Helper functions

// readFileWithValidation reads a file with validation
func (s *firmwareService) readFileWithValidation(filePath string) ([]byte, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found at path %s: %w", filePath, err)
	}
	
	// Read the file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	
	return data, nil
}

// verifySignature verifies the signature of a firmware release
func (s *firmwareService) verifySignature(release *models.FirmwareReleaseExtended) bool {
	// Check if signature algorithm is supported
	if release.SignatureAlgorithm != "ecdsa-secp256r1" {
		s.log.Warnf("Unsupported signature algorithm: %s", release.SignatureAlgorithm)
		return false
	}
	
	// Read the firmware file
	fileData, err := s.readFileWithValidation(release.FilePath)
	if err != nil {
		s.log.WithError(err).Error("Failed to read firmware file for signature verification")
		return false
	}
	
	// Hash the file
	hash := sha256.Sum256(fileData)
	
	// Parse the signature
	parts := strings.Split(release.Signature, ",")
	if len(parts) != 2 {
		s.log.Errorf("Invalid signature format: %s", release.Signature)
		return false
	}
	
	rHex, sHex := parts[0], parts[1]
	
	r, ok := new(big.Int).SetString(rHex, 16)
	if !ok {
		s.log.Errorf("Invalid signature r component: %s", rHex)
		return false
	}
	
	s, ok := new(big.Int).SetString(sHex, 16)
	if !ok {
		s.log.Errorf("Invalid signature s component: %s", sHex)
		return false
	}
	
	// Get the public key
	// In a real system, this would retrieve the key from a secure store
	// based on the certificate ID
	privateKey, ok := s.signatureKeys[release.CertificateID]
	if !ok {
		s.log.Errorf("Unknown certificate ID: %s", release.CertificateID)
		return false
	}
	
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	
	// Verify the signature
	return ecdsa.Verify(publicKey, hash[:], r, s)

}

// compareSemanticVersions compares two semantic versions
func (s *firmwareService) compareSemanticVersions(v1, v2 *models.SemanticVersion) (int, error) {
	// Compare major version
	if v1.Major < v2.Major {
		return -1, nil
	}
	if v1.Major > v2.Major {
		return 1, nil
	}
	
	// Compare minor version
	if v1.Minor < v2.Minor {
		return -1, nil
	}
	if v1.Minor > v2.Minor {
		return 1, nil
	}
	
	// Compare patch version
	if v1.Patch < v2.Patch {
		return -1, nil
	}
	if v1.Patch > v2.Patch {
		return 1, nil
	}
	
	// Compare pre-release (pre-release versions are lower than release versions)
	if v1.PreRelease == "" && v2.PreRelease != "" {
		return 1, nil
	}
	if v1.PreRelease != "" && v2.PreRelease == "" {
		return -1, nil
	}
	if v1.PreRelease < v2.PreRelease {
		return -1, nil
	}
	if v1.PreRelease > v2.PreRelease {
		return 1, nil
	}
	
	// Build metadata is ignored for comparison
	
	// Versions are equal
	return 0, nil
}