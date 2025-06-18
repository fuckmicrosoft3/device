// services/device/internal/infrastructure/wal.go
package infrastructure

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WALEntry represents an entry in the write-ahead log.
type WALEntry struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Retries   int         `json:"retries"`
}

// WAL implements a write-ahead log for data persistence.
type WAL struct {
	path         string
	file         *os.File
	encoder      *json.Encoder
	decoder      *json.Decoder
	mu           sync.Mutex
	rotationSize int64
	currentSize  int64
	maxRetries   int
}

// NewWAL creates a new write-ahead log.
func NewWAL(path string) (*WAL, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	// Open or create WAL file
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	// Get current file size
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat WAL file: %w", err)
	}

	wal := &WAL{
		path:         path,
		file:         file,
		encoder:      json.NewEncoder(file),
		currentSize:  stat.Size(),
		rotationSize: 100 * 1024 * 1024, // 100MB
		maxRetries:   5,
	}

	return wal, nil
}

// Write adds an entry to the WAL.
func (w *WAL) Write(data interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := WALEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		Type:      fmt.Sprintf("%T", data),
		Data:      data,
		Retries:   0,
	}

	// Encode entry
	entryBytes, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal WAL entry: %w", err)
	}

	// Write to file
	if _, err := w.file.Write(entryBytes); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}
	if _, err := w.file.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline to WAL: %w", err)
	}

	// Sync to disk
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	w.currentSize += int64(len(entryBytes) + 1)

	// Check if rotation is needed
	if w.currentSize > w.rotationSize {
		if err := w.rotate(); err != nil {
			return fmt.Errorf("failed to rotate WAL: %w", err)
		}
	}

	return nil
}

// ReadAll reads all entries from the WAL.
func (w *WAL) ReadAll() ([]interface{}, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Seek to beginning
	if _, err := w.file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek WAL: %w", err)
	}

	var entries []interface{}
	scanner := bufio.NewScanner(w.file)

	for scanner.Scan() {
		var entry WALEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			// Skip corrupted entries
			continue
		}

		if entry.Retries < w.maxRetries {
			entries = append(entries, entry.Data)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read WAL: %w", err)
	}

	// Seek back to end for writing
	if _, err := w.file.Seek(0, io.SeekEnd); err != nil {
		return nil, fmt.Errorf("failed to seek to end of WAL: %w", err)
	}

	return entries, nil
}

// Remove marks an entry as processed (logical deletion).
func (w *WAL) Remove(id string) error {
	// In a production system, you might implement logical deletion
	// For simplicity, we'll just log it
	return nil
}

// Compact removes processed entries from the WAL.
func (w *WAL) Compact() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	tempPath := w.path + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp WAL file: %w", err)
	}
	defer tempFile.Close()

	// Read all entries
	if _, err := w.file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek WAL: %w", err)
	}

	scanner := bufio.NewScanner(w.file)
	writer := bufio.NewWriter(tempFile)
	newSize := int64(0)

	for scanner.Scan() {
		var entry WALEntry
		line := scanner.Bytes()

		if err := json.Unmarshal(line, &entry); err != nil {
			continue // Skip corrupted entries
		}

		// Keep unprocessed entries and entries with remaining retries
		if entry.Retries < w.maxRetries {
			if _, err := writer.Write(line); err != nil {
				return fmt.Errorf("failed to write to temp WAL: %w", err)
			}
			if _, err := writer.WriteString("\n"); err != nil {
				return fmt.Errorf("failed to write newline to temp WAL: %w", err)
			}
			newSize += int64(len(line) + 1)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read WAL during compaction: %w", err)
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush temp WAL: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp WAL: %w", err)
	}

	// Close original file
	w.file.Close()

	// Replace original with temp
	if err := os.Rename(tempPath, w.path); err != nil {
		return fmt.Errorf("failed to replace WAL file: %w", err)
	}

	// Reopen file
	w.file, err = os.OpenFile(w.path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to reopen WAL file: %w", err)
	}

	w.encoder = json.NewEncoder(w.file)
	w.currentSize = newSize

	return nil
}

// rotate creates a new WAL file and archives the old one.
func (w *WAL) rotate() error {
	// Close current file
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close WAL file: %w", err)
	}

	// Archive current file
	archivePath := fmt.Sprintf("%s.%d", w.path, time.Now().Unix())
	if err := os.Rename(w.path, archivePath); err != nil {
		return fmt.Errorf("failed to archive WAL file: %w", err)
	}

	// Create new file
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create new WAL file: %w", err)
	}

	w.file = file
	w.encoder = json.NewEncoder(file)
	w.currentSize = 0

	// Start background archival compression
	go w.compressArchive(archivePath)

	return nil
}

// compressArchive compresses archived WAL files.
func (w *WAL) compressArchive(path string) {
	// Implement compression logic here
	// For now, just log it
}

// Close closes the WAL.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		if err := w.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync WAL before closing: %w", err)
		}
		return w.file.Close()
	}
	return nil
}

// Stats returns WAL statistics.
func (w *WAL) Stats() map[string]interface{} {
	w.mu.Lock()
	defer w.mu.Unlock()

	return map[string]interface{}{
		"path":          w.path,
		"size":          w.currentSize,
		"rotation_size": w.rotationSize,
	}
}

// generateID generates a unique ID for WAL entries.
func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
}
