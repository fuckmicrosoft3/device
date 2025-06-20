package enrichment

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// RetryItem represents an item in the retry queue
type RetryItem struct {
	SessionID  uuid.UUID
	RetryAfter time.Time
	Attempts   int
}

// RetryQueue manages failed enrichment retries
type RetryQueue struct {
	items         []*RetryItem
	mu            sync.RWMutex
	retryInterval time.Duration
	notify        chan struct{}
}

// NewRetryQueue creates a new retry queue
func NewRetryQueue(retryInterval time.Duration) *RetryQueue {
	return &RetryQueue{
		items:         make([]*RetryItem, 0),
		retryInterval: retryInterval,
		notify:        make(chan struct{}, 1),
	}
}

// Add adds an item to the retry queue
func (rq *RetryQueue) Add(item *RetryItem) {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	// Check if already exists
	for i, existing := range rq.items {
		if existing.SessionID == item.SessionID {
			// Update existing item
			rq.items[i] = item
			rq.notifyProcessor()
			return
		}
	}

	// Add new item
	rq.items = append(rq.items, item)
	rq.notifyProcessor()

	log.Debug().
		Str("session_id", item.SessionID.String()).
		Time("retry_after", item.RetryAfter).
		Int("attempts", item.Attempts).
		Msg("Added item to retry queue")
}

// GetDueItems returns items that are due for retry
func (rq *RetryQueue) GetDueItems() []*RetryItem {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	now := time.Now()
	dueItems := make([]*RetryItem, 0)
	remainingItems := make([]*RetryItem, 0)

	for _, item := range rq.items {
		if now.After(item.RetryAfter) {
			dueItems = append(dueItems, item)
		} else {
			remainingItems = append(remainingItems, item)
		}
	}

	// Update queue with remaining items
	rq.items = remainingItems

	return dueItems
}

// Remove removes an item from the queue
func (rq *RetryQueue) Remove(sessionID uuid.UUID) {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	newItems := make([]*RetryItem, 0, len(rq.items))
	for _, item := range rq.items {
		if item.SessionID != sessionID {
			newItems = append(newItems, item)
		}
	}

	rq.items = newItems
}

// Size returns the current queue size
func (rq *RetryQueue) Size() int {
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	return len(rq.items)
}

// Start starts the retry queue processor
func (rq *RetryQueue) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log.Info().Msg("Retry queue processor started")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Retry queue processor stopped")
			return
		case <-ticker.C:
			rq.processQueue()
		case <-rq.notify:
			rq.processQueue()
		}
	}
}

// processQueue checks for due items
func (rq *RetryQueue) processQueue() {
	rq.mu.RLock()
	queueSize := len(rq.items)
	rq.mu.RUnlock()

	if queueSize > 0 {
		log.Debug().Int("queue_size", queueSize).Msg("Processing retry queue")
	}
}

// notifyProcessor sends a notification to process the queue
func (rq *RetryQueue) notifyProcessor() {
	select {
	case rq.notify <- struct{}{}:
	default:
		// Channel is full, notification already pending
	}
}

// GetStats returns queue statistics
func (rq *RetryQueue) GetStats() map[string]interface{} {
	rq.mu.RLock()
	defer rq.mu.RUnlock()

	stats := map[string]interface{}{
		"queue_size": len(rq.items),
		"items":      make([]map[string]interface{}, 0),
	}

	for _, item := range rq.items {
		stats["items"] = append(stats["items"].([]map[string]interface{}), map[string]interface{}{
			"session_id":  item.SessionID.String(),
			"retry_after": item.RetryAfter,
			"attempts":    item.Attempts,
		})
	}

	return stats
}
