package ratelimiter_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-highload-demo/pkg/ratelimiter"
)

func setupRedis(t *testing.T) *ratelimiter.RedisAdapter {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set, skipping redis integration test")
	}

	adapter := ratelimiter.NewRedisAdapter(addr)
	require.NoError(t, adapter.Ping(context.Background()))
	t.Cleanup(func() { adapter.Close() })
	return adapter
}

// TestRedisAdapter_RateLimiting проверяет rate limiting через реальный Redis.
func TestRedisAdapter_RateLimiting(t *testing.T) {
	adapter := setupRedis(t)
	ctx := context.Background()
	limiter := ratelimiter.New(adapter, 3, time.Minute)

	key := "test:redis_adapter:" + t.Name()

	for i := 0; i < 3; i++ {
		allowed, err := limiter.Allow(ctx, key)
		require.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i+1)
	}

	// Четвёртый запрос — отклонён
	allowed, err := limiter.Allow(ctx, key)
	require.NoError(t, err)
	assert.False(t, allowed, "request 4 should be denied")
}
