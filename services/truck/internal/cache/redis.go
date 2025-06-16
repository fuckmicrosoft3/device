package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"example.com/backstage/services/truck/config"
	"example.com/backstage/services/truck/internal/model"
)

// CacheClient defines the interface for cache operations
type CacheClient interface {
	// Operation caching methods
	GetOperation(ctx context.Context, id string) (*model.Operation, error)
	SetOperation(ctx context.Context, operation *model.Operation) error
	DeleteOperation(ctx context.Context, id string) error
	
	// Operation group caching methods
	GetOperationGroup(ctx context.Context, id string) (*model.OperationGroup, error)
	SetOperationGroup(ctx context.Context, group *model.OperationGroup) error
	DeleteOperationGroup(ctx context.Context, id string) error
	
	// Get active operations by device MCU
	GetActiveOperationByDeviceMCU(ctx context.Context, mcu string) (*model.Operation, error)
	SetActiveOperationByDeviceMCU(ctx context.Context, mcu string, operation *model.Operation) error
	
	// Get active operation groups by transport MCU
	GetActiveOperationGroupByTransportMCU(ctx context.Context, mcu string) (*model.OperationGroup, error)
	SetActiveOperationGroupByTransportMCU(ctx context.Context, mcu string, group *model.OperationGroup) error
	
	// Clear all cache
	FlushAll(ctx context.Context) error
}

// RedisClient implements CacheClient using Redis
type RedisClient struct {
	client  *redis.Client
	enabled bool
	ttl     time.Duration
}

// NewRedisClient creates a new Redis client
func NewRedisClient(cfg *config.RedisConfig) (CacheClient, error) {
	if !cfg.Enabled {
		return &RedisClient{enabled: false}, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{
		client:  client,
		enabled: true,
		ttl:     time.Hour, // Default TTL
	}, nil
}

// Prefix keys to avoid collisions
func operationKey(id string) string {
	return fmt.Sprintf("operation:%s", id)
}

func operationGroupKey(id string) string {
	return fmt.Sprintf("operation_group:%s", id)
}

func activeOperationKey(mcu string) string {
	return fmt.Sprintf("active_operation:%s", mcu)
}

func activeOperationGroupKey(mcu string) string {
	return fmt.Sprintf("active_operation_group:%s", mcu)
}

// GetOperation retrieves an operation from cache
func (c *RedisClient) GetOperation(ctx context.Context, id string) (*model.Operation, error) {
	if !c.enabled {
		return nil, redis.Nil
	}

	data, err := c.client.Get(ctx, operationKey(id)).Bytes()
	if err != nil {
		return nil, err
	}

	var operation model.Operation
	if err := json.Unmarshal(data, &operation); err != nil {
		return nil, err
	}

	return &operation, nil
}

// SetOperation caches an operation
func (c *RedisClient) SetOperation(ctx context.Context, operation *model.Operation) error {
	if !c.enabled {
		return nil
	}

	data, err := json.Marshal(operation)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, operationKey(operation.UUID), data, c.ttl).Err()
}

// DeleteOperation removes an operation from cache
func (c *RedisClient) DeleteOperation(ctx context.Context, id string) error {
	if !c.enabled {
		return nil
	}

	return c.client.Del(ctx, operationKey(id)).Err()
}

// GetOperationGroup retrieves an operation group from cache
func (c *RedisClient) GetOperationGroup(ctx context.Context, id string) (*model.OperationGroup, error) {
	if !c.enabled {
		return nil, redis.Nil
	}

	data, err := c.client.Get(ctx, operationGroupKey(id)).Bytes()
	if err != nil {
		return nil, err
	}

	var group model.OperationGroup
	if err := json.Unmarshal(data, &group); err != nil {
		return nil, err
	}

	return &group, nil
}

// SetOperationGroup caches an operation group
func (c *RedisClient) SetOperationGroup(ctx context.Context, group *model.OperationGroup) error {
	if !c.enabled {
		return nil
	}

	data, err := json.Marshal(group)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, operationGroupKey(group.UUID), data, c.ttl).Err()
}

// DeleteOperationGroup removes an operation group from cache
func (c *RedisClient) DeleteOperationGroup(ctx context.Context, id string) error {
	if !c.enabled {
		return nil
	}

	return c.client.Del(ctx, operationGroupKey(id)).Err()
}

// GetActiveOperationByDeviceMCU retrieves an active operation for a device
func (c *RedisClient) GetActiveOperationByDeviceMCU(ctx context.Context, mcu string) (*model.Operation, error) {
	if !c.enabled {
		return nil, redis.Nil
	}

	data, err := c.client.Get(ctx, activeOperationKey(mcu)).Bytes()
	if err != nil {
		return nil, err
	}

	var operation model.Operation
	if err := json.Unmarshal(data, &operation); err != nil {
		return nil, err
	}

	return &operation, nil
}

// SetActiveOperationByDeviceMCU caches an active operation for a device
func (c *RedisClient) SetActiveOperationByDeviceMCU(ctx context.Context, mcu string, operation *model.Operation) error {
	if !c.enabled {
		return nil
	}

	data, err := json.Marshal(operation)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, activeOperationKey(mcu), data, c.ttl).Err()
}

// GetActiveOperationGroupByTransportMCU retrieves an active operation group for a transport
func (c *RedisClient) GetActiveOperationGroupByTransportMCU(ctx context.Context, mcu string) (*model.OperationGroup, error) {
	if !c.enabled {
		return nil, redis.Nil
	}

	data, err := c.client.Get(ctx, activeOperationGroupKey(mcu)).Bytes()
	if err != nil {
		return nil, err
	}

	var group model.OperationGroup
	if err := json.Unmarshal(data, &group); err != nil {
		return nil, err
	}

	return &group, nil
}

// SetActiveOperationGroupByTransportMCU caches an active operation group for a transport
func (c *RedisClient) SetActiveOperationGroupByTransportMCU(ctx context.Context, mcu string, group *model.OperationGroup) error {
	if !c.enabled {
		return nil
	}

	data, err := json.Marshal(group)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, activeOperationGroupKey(mcu), data, c.ttl).Err()
}

// FlushAll clears all cache
func (c *RedisClient) FlushAll(ctx context.Context) error {
	if !c.enabled {
		return nil
	}

	return c.client.FlushAll(ctx).Err()
}