// Package ratelimiter реализует ограничение частоты запросов (rate limiting)
// по алгоритму fixed window counter на базе Redis.
package ratelimiter

import (
	"context"
	"fmt"
	"time"
)

// RedisClient — интерфейс для взаимодействия с Redis.
// Позволяет подменять реализацию в тестах.
type RedisClient interface {
	// Incr атомарно увеличивает значение ключа на 1 и возвращает новое значение.
	Incr(ctx context.Context, key string) (int64, error)
	// Expire устанавливает TTL для ключа.
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

// RateLimiter ограничивает частоту запросов по ключу,
// используя алгоритм fixed window counter на базе Redis.
type RateLimiter struct {
	client RedisClient
	limit  int64
	window time.Duration
}

// New создаёт новый RateLimiter.
//   - client — Redis-клиент
//   - limit — максимальное количество запросов в окне
//   - window — длительность окна (TTL ключа)
func New(client RedisClient, limit int64, window time.Duration) *RateLimiter {
	return &RateLimiter{
		client: client,
		limit:  limit,
		window: window,
	}
}

// Allow проверяет, разрешён ли запрос для данного ключа.
// Возвращает true, если количество запросов не превышает лимит.
// При первом запросе (count == 1) устанавливает TTL на ключ.
func (r *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Incr(ctx, key)
	if err != nil {
		return false, fmt.Errorf("ratelimiter: incr failed: %w", err)
	}

	if count == 1 {
		if err := r.client.Expire(ctx, key, r.window); err != nil {
			return false, fmt.Errorf("ratelimiter: expire failed: %w", err)
		}
	}

	return count <= r.limit, nil
}
