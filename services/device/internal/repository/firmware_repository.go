package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"example.com/backstage/services/device/internal/models"
	"gorm.io/gorm"
)

// Implementation of FirmwareRepository interface

// GetFirmwareReleaseByID retrieves a firmware release by ID
func (r *firmwareRepo) GetFirmwareReleaseByID(ctx context.Context, id uint) (*models.FirmwareReleaseExtended, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	var release models.FirmwareReleaseExtended
	if err := gormDB.Preload("TestRelease").Preload("TestDevice").Preload("ProductionRelease").First(&release, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("firmware release with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get firmware release: %w", err)
	}

	return &release, nil
}

// CreateFirmwareRelease creates a new firmware release
func (r *firmwareRepo) CreateFirmwareRelease(ctx context.Context, release *models.FirmwareReleaseExtended) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	// Run in transaction to ensure atomicity
	return gormDB.Transaction(func(tx *gorm.DB) error {
		// First create the base firmware release
		baseRelease := &models.FirmwareRelease{
			FilePath:      release.FilePath,
			ReleaseType:   release.ReleaseType,
			Version:       release.Version,
			Active:        release.Active,
			Size:          release.Size,
			Valid:         release.Valid,
			IsTest:        release.IsTest,
			TestReleaseID: release.TestReleaseID,
			TestDeviceID:  release.TestDeviceID,
			TestPassed:    release.TestPassed,
			FileHash:      release.FileHash,
		}

		if err := tx.Create(baseRelease).Error; err != nil {
			return fmt.Errorf("failed to create base firmware release: %w", err)
		}

		// Set the ID of the embedded firmware release to the new one
		release.Model = baseRelease.Model

		// Now create the extended release with a reference to the base release
		if err := tx.Create(release).Error; err != nil {
			return fmt.Errorf("failed to create extended firmware release: %w", err)
		}

		return nil
	})
}

// UpdateFirmwareRelease updates an existing firmware release
func (r *firmwareRepo) UpdateFirmwareRelease(ctx context.Context, release *models.FirmwareReleaseExtended) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	// Run in transaction to ensure atomicity
	return gormDB.Transaction(func(tx *gorm.DB) error {
		// First update the base firmware release
		baseRelease := &models.FirmwareRelease{
			Model:         release.Model,
			FilePath:      release.FilePath,
			ReleaseType:   release.ReleaseType,
			Version:       release.Version,
			Active:        release.Active,
			Size:          release.Size,
			Valid:         release.Valid,
			IsTest:        release.IsTest,
			TestReleaseID: release.TestReleaseID,
			TestDeviceID:  release.TestDeviceID,
			TestPassed:    release.TestPassed,
			FileHash:      release.FileHash,
		}

		if err := tx.Save(baseRelease).Error; err != nil {
			return fmt.Errorf("failed to update base firmware release: %w", err)
		}

		// Now update the extended release
		if err := tx.Save(release).Error; err != nil {
			return fmt.Errorf("failed to update extended firmware release: %w", err)
		}

		return nil
	})
}

// GetFirmwareReleaseByVersion gets a firmware release by version
func (r *firmwareRepo) GetFirmwareReleaseByVersion(ctx context.Context, version string, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	var release models.FirmwareReleaseExtended
	query := gormDB.Preload("TestRelease").Preload("TestDevice").Preload("ProductionRelease")
	
	// Apply filters
	query = query.Where("version = ?", version)
	if releaseType != "" {
		query = query.Where("release_type = ?", releaseType)
	}
	
	if err := query.First(&release).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("firmware release with version %s and type %s not found", version, releaseType)
		}
		return nil, fmt.Errorf("failed to get firmware release: %w", err)
	}

	return &release, nil
}

// ListFirmwareReleases lists firmware releases with optional filtering by type
func (r *firmwareRepo) ListFirmwareReleases(ctx context.Context, releaseType models.ReleaseType) ([]*models.FirmwareReleaseExtended, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	var releases []*models.FirmwareReleaseExtended
	query := gormDB.Preload("TestRelease").Preload("ProductionRelease").
		Order("major_version DESC, minor_version DESC, patch_version DESC")

	if releaseType != "" {
		query = query.Where("release_type = ?", releaseType)
	}

	if err := query.Find(&releases).Error; err != nil {
		return nil, fmt.Errorf("failed to list firmware releases: %w", err)
	}

	return releases, nil
}

// GetLatestFirmwareRelease gets the latest firmware release
func (r *firmwareRepo) GetLatestFirmwareRelease(ctx context.Context, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	var release models.FirmwareReleaseExtended
	query := gormDB.Preload("TestRelease").Preload("ProductionRelease").
		Where("active = ? AND valid = ?", true, true)

	if releaseType != "" {
		query = query.Where("release_type = ?", releaseType)
	}

	if err := query.Order("major_version DESC, minor_version DESC, patch_version DESC").
		First(&release).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no active firmware releases found for type %s", releaseType)
		}
		return nil, fmt.Errorf("failed to get latest firmware release: %w", err)
	}

	return &release, nil
}

// CreateFirmwareValidation creates a validation record for a firmware release
func (r *firmwareRepo) CreateFirmwareValidation(ctx context.Context, validation *models.FirmwareReleaseValidation) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	if err := gormDB.Create(validation).Error; err != nil {
		return fmt.Errorf("failed to create firmware validation: %w", err)
	}

	return nil
}

// ListValidFirmwareReleases lists valid firmware releases
func (r *firmwareRepo) ListValidFirmwareReleases(ctx context.Context, releaseType models.ReleaseType, activeOnly bool) ([]*models.FirmwareReleaseExtended, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	var releases []*models.FirmwareReleaseExtended
	query := gormDB.Preload("TestRelease").Preload("ProductionRelease").
		Where("valid = ?", true)

	if releaseType != "" {
		query = query.Where("release_type = ?", releaseType)
	}

	if activeOnly {
		query = query.Where("active = ?", true)
	}

	if err := query.Order("major_version DESC, minor_version DESC, patch_version DESC").
		Find(&releases).Error; err != nil {
		return nil, fmt.Errorf("failed to list valid firmware releases: %w", err)
	}

	return releases, nil
}

// GetFirmwareManifest gets a firmware manifest by ID
func (r *firmwareRepo) GetFirmwareManifest(ctx context.Context, id uint) (*models.FirmwareManifest, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	var manifest models.FirmwareManifest
	if err := gormDB.Preload("Releases").First(&manifest, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("firmware manifest with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get firmware manifest: %w", err)
	}

	return &manifest, nil
}

// CreateFirmwareManifest creates a new firmware manifest
func (r *firmwareRepo) CreateFirmwareManifest(ctx context.Context, manifest *models.FirmwareManifest, releases []*models.FirmwareReleaseExtended) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	// Run in transaction
	return gormDB.Transaction(func(tx *gorm.DB) error {
		// First create the manifest
		if err := tx.Create(manifest).Error; err != nil {
			return fmt.Errorf("failed to create firmware manifest: %w", err)
		}

		// Convert extended releases to base releases for the association
		baseReleases := make([]models.FirmwareRelease, len(releases))
		for i, release := range releases {
			baseReleases[i] = release.FirmwareRelease
		}

		// Associate releases with manifest
		if err := tx.Model(manifest).Association("Releases").Append(baseReleases); err != nil {
			return fmt.Errorf("failed to associate releases with manifest: %w", err)
		}

		return nil
	})
}

// UpdateFirmwareManifest updates an existing firmware manifest
func (r *firmwareRepo) UpdateFirmwareManifest(ctx context.Context, manifest *models.FirmwareManifest) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	// Run in transaction to handle associations
	return gormDB.Transaction(func(tx *gorm.DB) error {
		// First clear existing release associations
		if err := tx.Model(manifest).Association("Releases").Clear(); err != nil {
			return fmt.Errorf("failed to clear manifest releases: %w", err)
		}

		// Update manifest fields
		manifest.GeneratedAt = time.Now()
		if err := tx.Save(manifest).Error; err != nil {
			return fmt.Errorf("failed to save manifest: %w", err)
		}

		// Re-add release associations
		if err := tx.Model(manifest).Association("Releases").Replace(manifest.Releases); err != nil {
			return fmt.Errorf("failed to update manifest releases: %w", err)
		}

		return nil
	})
}