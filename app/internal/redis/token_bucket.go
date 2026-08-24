package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// TokenBucketConfig configures a Redis-backed token bucket.
type TokenBucketConfig struct {
	Client         *goredis.Client
	Capacity       int64
	RefillRate     int64
	RefillInterval time.Duration
}

// TokenBucket limits requests with an atomic Redis Lua script.
type TokenBucket struct {
	client         *goredis.Client
	capacity       int64
	refillRate     int64
	refillInterval time.Duration
}

func NewTokenBucket(config TokenBucketConfig) *TokenBucket {
	return &TokenBucket{
		client:         config.Client,
		capacity:       config.Capacity,
		refillRate:     config.RefillRate,
		refillInterval: config.RefillInterval,
	}
}

var tokenBucketScript = goredis.NewScript(`
local bucket = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local refill_interval = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

local stored = redis.call('HMGET', bucket, 'tokens', 'updated_at')
local tokens = tonumber(stored[1]) or capacity
local updated_at = tonumber(stored[2]) or now
tokens = math.min(capacity, tokens + (now - updated_at) * refill_rate / refill_interval)

local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end

redis.call('HSET', bucket, 'tokens', tokens, 'updated_at', now)
redis.call('PEXPIRE', bucket, math.ceil(capacity * refill_interval / refill_rate))
return {allowed, string.format('%.6f', tokens)}
`)

// Allow consumes one token for key and reports whether it was available.
func (b *TokenBucket) Allow(ctx context.Context, key string) (bool, float64, error) {
	if b.client == nil || b.capacity <= 0 || b.refillRate <= 0 || b.refillInterval <= 0 {
		return false, 0, fmt.Errorf("invalid token bucket configuration")
	}

	result, err := tokenBucketScript.Run(
		ctx, b.client,
		[]string{key},
		b.capacity,
		b.refillRate,
		b.refillInterval.Milliseconds(),
		time.Now().UnixMilli(),
	).Result()
	
	if err != nil {
		return false, 0, err
	}

	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return false, 0, fmt.Errorf("unexpected token bucket result: %T", result)
	}
	allowed, err := strconv.ParseInt(fmt.Sprint(values[0]), 10, 64)
	if err != nil {
		return false, 0, fmt.Errorf("parse token bucket allowance: %w", err)
	}
	remaining, err := strconv.ParseFloat(fmt.Sprint(values[1]), 64)
	if err != nil {
		return false, 0, fmt.Errorf("parse remaining tokens: %w", err)
	}
	return allowed == 1, remaining, nil
}
