// Package cache is a small key-value cache with TTL on top of Redis.
package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const pingTimeout = 2 * time.Second

type Cache struct {
	client *redis.Client
}

func addr() string {
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		return v
	}
	return "localhost:6379"
}

// Describe returns the Redis address, for logs.
func Describe() string { return addr() }

// Open connects to Redis using REDIS_ADDR (default localhost:6379),
// REDIS_DB (default 0) and the optional REDIS_PASSWORD.
func Open(ctx context.Context) (*Cache, error) {
	db := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("REDIS_DB must be a non-negative number, got %q", v)
		}
		db = n
	}
	client := redis.NewClient(&redis.Options{
		Addr:     addr(),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Cache{client: client}, nil
}

func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// DeletePrefix removes every key that starts with prefix.
func (c *Cache) DeletePrefix(ctx context.Context, prefix string) error {
	iter := c.client.Scan(ctx, 0, prefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

func (c *Cache) Close() error { return c.client.Close() }
