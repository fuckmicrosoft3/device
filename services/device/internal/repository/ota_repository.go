package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"example.com/backstage/services/device/internal/models"
	"gorm.io/gorm"
)

// Implementation of OTARepository interface

// GetUpdateSession gets an update session by ID
func (r *otaRepo) GetUpdateSession(ctx context.Context, sessionID string) (*models.OTAUpdateSession, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	var session models.OTAUpdateSession
	if err := gormDB.Preload("Device").Preload("FirmwareRelease").
		Where("session_id = ?", sessionID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("update session with ID %s not found", sessionID)
		}
		return nil, fmt.Errorf("failed to get update session: %w", err)
	}

	return &session, nil
}

// CreateUpdateSession creates a new update session
func (r *otaRepo) CreateUpdateSession(ctx context.Context, session *models.OTAUpdateSession) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	if err := gormDB.Create(session).Error; err != nil {
		return fmt.Errorf("failed to create update session: %w", err)
	}

	return nil
}

// UpdateUpdateSession updates an existing update session
func (r *otaRepo) UpdateUpdateSession(ctx context.Context, session *models.OTAUpdateSession) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	if err := gormDB.Save(session).Error; err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// ListDeviceUpdateSessions lists update sessions for a device
func (r *otaRepo) ListDeviceUpdateSessions(ctx context.Context, deviceID uint, limit int) ([]*models.OTAUpdateSession, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	var sessions []*models.OTAUpdateSession
	query := gormDB.Preload("FirmwareRelease").
		Where("device_id = ?", deviceID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to list device update sessions: %w", err)
	}

	return sessions, nil
}

// GetPendingUpdateSessions gets pending update sessions for a device
func (r *otaRepo) GetPendingUpdateSessions(ctx context.Context, deviceID uint) ([]*models.OTAUpdateSession, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	var sessions []*models.OTAUpdateSession
	if err := gormDB.Preload("FirmwareRelease").
		Where("device_id = ? AND status IN ?", deviceID, []models.OTAUpdateStatus{
			models.OTAStatusScheduled,
			models.OTAStatusPending,
		}).
		Order("priority DESC, created_at ASC").
		Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to get pending update sessions: %w", err)
	}

	return sessions, nil
}

// GetUpdateBatch gets a batch by ID
func (r *otaRepo) GetUpdateBatch(ctx context.Context, batchID string) (*models.OTAUpdateBatch, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	var batch models.OTAUpdateBatch
	if err := gormDB.Preload("FirmwareRelease").
		Where("batch_id = ?", batchID).First(&batch).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("update batch with ID %s not found", batchID)
		}
		return nil, fmt.Errorf("failed to get update batch: %w", err)
	}

	return &batch, nil
}

// CreateUpdateBatch creates a new update batch
func (r *otaRepo) CreateUpdateBatch(ctx context.Context, batch *models.OTAUpdateBatch) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	// Run in transaction
	return gormDB.Transaction(func(tx *gorm.DB) error {
		// Save the batch first
		if err := tx.Create(batch).Error; err != nil {
			return fmt.Errorf("failed to create update batch: %w", err)
		}

		// Process sessions if any
		if len(batch.Sessions) > 0 {
			// Create in batches of 100
			batchSize := 100
			for i := 0; i < len(batch.Sessions); i += batchSize {
				end := i + batchSize
				if end > len(batch.Sessions) {
					end = len(batch.Sessions)
				}

				if err := tx.Create(batch.Sessions[i:end]).Error; err != nil {
					return fmt.Errorf("failed to create update sessions batch %d-%d: %w", i, end, err)
				}
			}
		}

		return nil
	})
}

// UpdateUpdateBatch updates an existing update batch
func (r *otaRepo) UpdateUpdateBatch(ctx context.Context, batch *models.OTAUpdateBatch) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	if err := gormDB.Save(batch).Error; err != nil {
		return fmt.Errorf("failed to update batch: %w", err)
	}

	return nil
}

// ListBatchUpdateSessions lists update sessions in a batch
func (r *otaRepo) ListBatchUpdateSessions(ctx context.Context, batchID string) ([]*models.OTAUpdateSession, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	var sessions []*models.OTAUpdateSession
	if err := gormDB.Preload("Device").Preload("FirmwareRelease").
		Where("batch_id = ?", batchID).Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to list batch update sessions: %w", err)
	}

	return sessions, nil
}

// CreateDeviceLog creates a device log entry
func (r *otaRepo) CreateDeviceLog(ctx context.Context, log *models.OTADeviceLog) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	if err := gormDB.Create(log).Error; err != nil {
		return fmt.Errorf("failed to create device log: %w", err)
	}

	return nil
}

// GetStuckUpdateSessions gets stuck update sessions
func (r *otaRepo) GetStuckUpdateSessions(ctx context.Context, threshold time.Time) ([]*models.OTAUpdateSession, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	var staleSessions []*models.OTAUpdateSession
	
	// Find sessions that have been stuck in an intermediate state for too long
	if err := gormDB.Preload("Device").Preload("FirmwareRelease").
		Where(`(status = ? AND download_started_at < ?) OR 
			  (status = ? AND acknowledged_at < ?) OR 
			  (status = ? AND verification_started_at < ?) OR
			  (status = ? AND install_started_at < ?)`,
			models.OTAStatusDownloading, threshold,
			models.OTAStatusAcknowledged, threshold,
			models.OTAStatusVerifying, threshold,
			models.OTAStatusInstalling, threshold).
		Find(&staleSessions).Error; err != nil {
		return nil, fmt.Errorf("failed to get stuck update sessions: %w", err)
	}
	
	return staleSessions, nil
}

// GetUpdateStats gets statistics about updates
func (r *otaRepo) GetUpdateStats(ctx context.Context) (map[string]interface{}, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	stats := make(map[string]interface{})
	
	// Get count of sessions by status
	var statusCounts []struct {
		Status string
		Count  int
	}
	
	if err := gormDB.Model(&models.OTAUpdateSession{}).
		Select("status, count(*) as count").
		Group("status").
		Find(&statusCounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get status counts: %w", err)
	}
	
	// Convert to map
	statusCountMap := make(map[string]int)
	for _, sc := range statusCounts {
		statusCountMap[sc.Status] = sc.Count
	}
	stats["status_counts"] = statusCountMap
	
	// Get count of batches by status
	var batchStatusCounts []struct {
		Status string
		Count  int
	}
	
	if err := gormDB.Model(&models.OTAUpdateBatch{}).
		Select("status, count(*) as count").
		Group("status").
		Find(&batchStatusCounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get batch status counts: %w", err)
	}
	
	// Convert to map
	batchStatusCountMap := make(map[string]int)
	for _, sc := range batchStatusCounts {
		batchStatusCountMap[sc.Status] = sc.Count
	}
	stats["batch_status_counts"] = batchStatusCountMap
	
	// Get total sessions and batches
	var totalSessions, totalBatches int64
	
	if err := gormDB.Model(&models.OTAUpdateSession{}).Count(&totalSessions).Error; err != nil {
		return nil, fmt.Errorf("failed to get total sessions: %w", err)
	}
	stats["total_sessions"] = totalSessions
	
	if err := gormDB.Model(&models.OTAUpdateBatch{}).Count(&totalBatches).Error; err != nil {
		return nil, fmt.Errorf("failed to get total batches: %w", err)
	}
	stats["total_batches"] = totalBatches
	
	// Get recent activity
	var recentActivity []struct {
		Date  string
		Count int
	}
	
	if err := gormDB.Model(&models.OTAUpdateSession{}).
		Select("DATE(created_at) as date, count(*) as count").
		Where("created_at > ?", time.Now().AddDate(0, 0, -30)).
		Group("DATE(created_at)").
		Order("date DESC").
		Find(&recentActivity).Error; err != nil {
		return nil, fmt.Errorf("failed to get recent activity: %w", err)
	}
	
	stats["recent_activity"] = recentActivity
	
	return stats, nil
}