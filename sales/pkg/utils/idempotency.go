package utils

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog/log"
)

// IdempotencyCache provides in-memory caching for idempotent operations
type IdempotencyCache struct {
	cache *lru.Cache[uuid.UUID, CacheEntry]
	mu    sync.RWMutex
}

// CacheEntry represents a cached idempotency result
type CacheEntry struct {
	Result    interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
}

// NewIdempotencyCache creates a new idempotency cache
func NewIdempotencyCache(size int) (*IdempotencyCache, error) {
	cache, err := lru.New[uuid.UUID, CacheEntry](size)
	if err != nil {
		return nil, err
	}

	ic := &IdempotencyCache{
		cache: cache,
	}

	// Start cleanup goroutine
	go ic.cleanupExpired()

	return ic, nil
}

// Get retrieves a cached result
func (ic *IdempotencyCache) Get(key uuid.UUID) (interface{}, bool) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	entry, found := ic.cache.Get(key)
	if !found {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		ic.cache.Remove(key)
		return nil, false
	}

	return entry.Result, true
}

// Set stores a result in the cache
func (ic *IdempotencyCache) Set(key uuid.UUID, result interface{}, ttl time.Duration) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	entry := CacheEntry{
		Result:    result,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}

	ic.cache.Add(key, entry)
}

// cleanupExpired periodically removes expired entries
func (ic *IdempotencyCache) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ic.mu.Lock()
		keys := ic.cache.Keys()
		for _, key := range keys {
			if entry, ok := ic.cache.Get(key); ok {
				if time.Now().After(entry.ExpiresAt) {
					ic.cache.Remove(key)
				}
			}
		}
		ic.mu.Unlock()
	}
}

// MessageDeduplicator prevents duplicate message processing
type MessageDeduplicator struct {
	processed sync.Map
	ttl       time.Duration
}

// NewMessageDeduplicator creates a new message deduplicator
func NewMessageDeduplicator(ttl time.Duration) *MessageDeduplicator {
	md := &MessageDeduplicator{
		ttl: ttl,
	}

	// Start cleanup goroutine
	go md.cleanup()

	return md
}

// ProcessOnce ensures a message is processed only once
func (md *MessageDeduplicator) ProcessOnce(messageID string, process func() error) error {
	// Try to mark as processing
	entry := &dedupeEntry{
		processedAt: time.Now(),
		expiresAt:   time.Now().Add(md.ttl),
	}

	if loaded, _ := md.processed.LoadOrStore(messageID, entry); loaded {
		// Already processed
		log.Debug().Str("message_id", messageID).Msg("Message already processed, skipping")
		return nil
	}

	// Process the message
	if err := process(); err != nil {
		// Remove from processed on error so it can be retried
		md.processed.Delete(messageID)
		return err
	}

	return nil
}

type dedupeEntry struct {
	processedAt time.Time
	expiresAt   time.Time
}

// cleanup periodically removes expired entries
func (md *MessageDeduplicator) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		md.processed.Range(func(key, value interface{}) bool {
			if entry, ok := value.(*dedupeEntry); ok {
				if now.After(entry.expiresAt) {
					md.processed.Delete(key)
				}
			}
			return true
		})
	}
}

// IdempotentOperation provides a helper for idempotent operations
type IdempotentOperation struct {
	cache *IdempotencyCache
}

// NewIdempotentOperation creates a new idempotent operation handler
func NewIdempotentOperation(cacheSize int) (*IdempotentOperation, error) {
	cache, err := NewIdempotencyCache(cacheSize)
	if err != nil {
		return nil, err
	}

	return &IdempotentOperation{
		cache: cache,
	}, nil
}

// Execute performs an idempotent operation
func (io *IdempotentOperation) Execute(ctx context.Context, key uuid.UUID, operation func() (interface{}, error)) (interface{}, error) {
	// Check cache first
	if result, found := io.cache.Get(key); found {
		log.Debug().Str("idempotency_key", key.String()).Msg("Returning cached result")
		return result, nil
	}

	// Execute operation
	result, err := operation()
	if err != nil {
		return nil, err
	}

	// Cache successful result
	io.cache.Set(key, result, 5*time.Minute)

	return result, nil
}
