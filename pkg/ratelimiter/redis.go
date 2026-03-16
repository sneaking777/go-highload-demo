package ratelimiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisAdapter адаптирует go-redis клиент к интерфейсу RedisClient.
type RedisAdapter struct {
	client *redis.Client
}

// NewRedisAdapter создаёт адаптер для go-redis клиента.
func NewRedisAdapter(addr string) *RedisAdapter {
	return &RedisAdapter{
		client: redis.NewClient(&redis.Options{Addr: addr}),
	}
}

// Ping проверяет соединение с Redis.
func (a *RedisAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx).Err()
}

// Close закрывает соединение с Redis.
func (a *RedisAdapter) Close() error {
	return a.client.Close()
}

// Incr атомарно увеличивает значение ключа на 1 и возвращает новое значение.
func (a *RedisAdapter) Incr(ctx context.Context, key string) (int64, error) {
	return a.client.Incr(ctx, key).Result()
}

// Expire устанавливает TTL для ключа.
func (a *RedisAdapter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return a.client.Expire(ctx, key, ttl).Err()
}
