package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"example.com/backstage/services/sales/config"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// RedisCache provides caching using Redis
type RedisCache struct {
	client  *redis.Client
	enabled bool
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(cfg config.RedisConfig) (*RedisCache, error) {
	if !cfg.Enabled {
		return &RedisCache{enabled: false}, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Redis")
	}

	return &RedisCache{
		client:  client,
		enabled: true,
	}, nil
}

// Get retrieves a value from cache
func (c *RedisCache) Get(ctx context.Context, key string, value interface{}) error {
	if !c.enabled {
		return errors.New("cache is disabled")
	}

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return errors.Wrap(err, "key not found in cache")
		}
		return errors.Wrap(err, "failed to get value from Redis")
	}

	err = json.Unmarshal(data, value)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal cached value")
	}

	return nil
}

// Set stores a value in cache with optional expiration
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if !c.enabled {
		return errors.New("cache is disabled")
	}

	data, err := json.Marshal(value)
	if err != nil {
		return errors.Wrap(err, "failed to marshal value for caching")
	}

	err = c.client.Set(ctx, key, data, expiration).Err()
	if err != nil {
		return errors.Wrap(err, "failed to set value in Redis")
	}

	return nil
}

// GetMachineCacheKey generates a cache key for machine data
func GetMachineCacheKey(id uuid.UUID) string {
	return fmt.Sprintf("machine:%s", id.String())
}

// GetDeviceCacheKey generates a cache key for device data
func GetDeviceCacheKey(mcu string) string {
	return fmt.Sprintf("device:%s", mcu)
}

// GetRevisionCacheKey generates a cache key for machine revision data
func GetRevisionCacheKey(id uuid.UUID) string {
	return fmt.Sprintf("revision:%s", id.String())
}

// GetTenantCacheKey generates a cache key for tenant data
func GetTenantCacheKey(id uuid.UUID) string {
	return fmt.Sprintf("tenant:%s", id.String())
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	if !c.enabled || c.client == nil {
		return nil
	}
	
	return c.client.Close()
}