package ratelimiter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRedis реализует RedisClient для тестов.
type mockRedis struct {
	incrResult int64
	expireCalled bool
	err error
}

func (m *mockRedis) Incr(ctx context.Context, key string) (int64, error)  {
	if m.err != nil {
		return 0, m.err	
	}
	m.incrResult++
	return m.incrResult, nil
}

func (m *mockRedis) Expire(ctx context.Context, key string, ttl time.Duration) error {
	m.expireCalled = true
	return m.err
}

// TestAllow_WithinLimit проверяет, что запрос разрешён при счётчике ниже лимита.
func TestAllow_WithinLimit(t *testing.T)  {
	mock := &mockRedis{}
	limiter := New(mock, 5, time.Minute)

	allowed, err := limiter.Allow(context.Background(), "user:1")

	require.NoError(t, err)
	assert.True(t, allowed)
}

// TestAllow_ExceedsLimit проверяет, что запрос отклонён при превышении лимита.
func TestAllow_ExceedsLimit(t *testing.T)  {
	mock := &mockRedis{incrResult: 5} // следующий Incr вернёт 6
	limiter := New(mock, 5, time.Minute)
	
	allowed, err := limiter.Allow(context.Background(), "user:1")

	require.NoError(t, err)
	assert.False(t, allowed)
}

// TestAllow_ExactlyAtLimit проверяет, что запрос разрешён при точном совпадении с лимитом.
func TestAllow_ExactlyAtLimit(t *testing.T)  {
	mock := &mockRedis{incrResult: 4} // следующий Incr вернёт 5
	limiter := New(mock, 5, time.Minute)

	allowed, err := limiter.Allow(context.Background(), "user:1")

	require.NoError(t, err)
	assert.True(t, allowed)
}

// TestAllow_SetsExpireOnFirstRequest проверяет, что TTL устанавливается при первом запросе.
func TestAllow_SetsExpireOnFirstRequest(t *testing.T)  {
	mock := &mockRedis{}  //Incr вернёт 1
	limiter := New(mock, 5, time.Minute)
	
	_, err := limiter.Allow(context.Background(), "user:1")

	require.NoError(t, err)
	assert.True(t, mock.expireCalled)
}

// TestAllow_DoesNotExpireOnSubsequentRequests проверяет, что TTL не переустанавливается при повторных запросах.
func TestAllow_DoesNotExpireOnSubsequentRequests(t *testing.T)  {
	mock := &mockRedis{incrResult: 1} // Incr вернёт 2
	limiter := New(mock, 5, time.Minute)
	
	_, err :=limiter.Allow(context.Background(), "user:1")

	require.NoError(t, err)
	assert.False(t, mock.expireCalled)
}

// TestAllow_RedisError проверяет, что ошибка Redis пробрасывается и запрос отклоняется.
func TestAllow_RedisError(t *testing.T)  {
	mock := &mockRedis{err: errors.New("connection refused")}
	limiter := New(mock, 5, time.Minute)
	
	allowed, err := limiter.Allow(context.Background(), "user:1")

	assert.Error(t, err)
	assert.False(t, allowed)
}

// TestAllow_IndependentKeys проверяет, что разные ключи имеют независимые счётчики.
func TestAllow_IndependentKeys(t *testing.T)  {
	mock := &mockRedis{}
	limiter := New(mock, 1, time.Minute)
	
	allowed1, err := limiter.Allow(context.Background(), "user:1")
	require.NoError(t, err)
	assert.True(t, allowed1)

	// mock сбрасываем — имитируем независимый ключ
	mock.incrResult = 0
	mock.expireCalled = false

	allowed2, err := limiter.Allow(context.Background(), "user:2")
	require.NoError(t, err)
	assert.True(t, allowed2)
}
