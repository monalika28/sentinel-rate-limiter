package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Sliding Window Counter implementation via Atomic Lua Script
var slidingWindowLua = redis.NewScript(`
	local key = KEYS[1]
	local now = tonumber(ARGV[1])
	local window = tonumber(ARGV[2])
	local limit = tonumber(ARGV[3])
	local clear_before = now - window

	-- 1. Remove expired timestamps outside the current window
	redis.call('ZREMRANGEBYSCORE', key, 0, clear_before)

	-- 2. Count active requests within current sliding window
	local current_requests = redis.call('ZCARD', key)

	if current_requests < limit then
		-- Allowed: Record current timestamp with millisecond score
		redis.call('ZADD', key, now, now)
		-- Set safety TTL so abandoned keys auto-expire
		redis.call('EXPIRE', key, math.ceil(window / 1000) + 1)
		return {1, limit - current_requests - 1, 0}
	else
		-- Throttled: Calculate exact retry_after in seconds
		local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
		local retry_after = 1
		if #oldest > 0 then
			retry_after = math.ceil((tonumber(oldest[2]) + window - now) / 1000)
			if retry_after <= 0 then
				retry_after = 1
			end
		end
		return {0, 0, retry_after}
	end
`)

type RedisStore struct {
	client *redis.Client
}

type RateLimitResult struct {
	Allowed    bool
	Remaining  int64
	RetryAfter int64
}

// NewRedisStore initializes connection pool for high-concurrency throughput
func NewRedisStore(addr, password string, db int) (*RedisStore, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     100, // production pool for 10k+ RPS
		MinIdleConns: 10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &RedisStore{client: rdb}, nil
}

// CheckLimit executes the atomic Lua evaluation on the Redis cluster
func (s *RedisStore) CheckLimit(ctx context.Context, key string, limit int, window time.Duration) (*RateLimitResult, error) {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	windowMs := window.Milliseconds()

	res, err := slidingWindowLua.Run(ctx, s.client, []string{key}, now, windowMs, limit).Slice()
	if err != nil {
		return nil, err
	}

	allowed := res[0].(int64) == 1
	remaining := res[1].(int64)
	retryAfter := res[2].(int64)

	return &RateLimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		RetryAfter: retryAfter,
	}, nil
}