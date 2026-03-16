// Package main — точка входа Notification Service.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-highload-demo/internal/app"
	"github.com/go-highload-demo/internal/broker"
	"github.com/go-highload-demo/internal/config"
	"github.com/go-highload-demo/internal/repository"
	"github.com/go-highload-demo/internal/service"
	"github.com/go-highload-demo/internal/storage"
	"github.com/go-highload-demo/pkg/ratelimiter"
)

// noopLimiter — заглушка rate limiter при отсутствии Redis.
type noopLimiter struct{}

// Allow всегда разрешает запрос.
func (n *noopLimiter) Allow(_ context.Context, _ string) (bool, error) {
	return true, nil
}

// main инициализирует зависимости и запускает HTTP-сервер с graceful shutdown.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	store, closeStore := initStore(ctx, cfg)
	if closeStore != nil {
		defer closeStore()
	}

	repo := repository.New(store)
	limiter, closeRedis := initLimiter(ctx, cfg)
	if closeRedis != nil {
		defer closeRedis()
	}

	brk, closeBroker := initBroker(cfg)
	if closeBroker != nil {
		defer closeBroker()
	}

	a := app.New(cfg, repo, limiter, brk)
	addr, err := a.Start()
	if err != nil {
		log.Fatalf("start: %v", err)
	}
	log.Printf("server started on %s", addr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	if err := a.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Println("server stopped")
}

// initStore создаёт хранилище: PostgreSQL если DB_HOST задан, иначе in-memory.
func initStore(ctx context.Context, cfg *config.Config) (repository.Store, func()) {
	dsn := cfg.Postgres.DSN()
	if dsn == "" {
		log.Println("DB_HOST not set, using in-memory store")
		return storage.NewMemoryStore(), nil
	}

	pg, err := storage.NewPostgresStore(ctx, dsn)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}

	migrationSQL, err := os.ReadFile("migrations/001_create_notifications.sql")
	if err != nil {
		log.Fatalf("read migration: %v", err)
	}
	if err := pg.Migrate(ctx, string(migrationSQL)); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("postgres connected, migrations applied")

	return pg, pg.Close
}

// initLimiter создаёт rate limiter: Redis если REDIS_ADDR задан, иначе noop.
func initLimiter(ctx context.Context, cfg *config.Config) (service.RateLimiter, func()) {
	if cfg.Redis.Addr == "" {
		log.Println("REDIS_ADDR not set, rate limiting disabled")
		return &noopLimiter{}, nil
	}

	adapter := ratelimiter.NewRedisAdapter(cfg.Redis.Addr)
	if err := adapter.Ping(ctx); err != nil {
		log.Fatalf("redis: %v", err)
	}
	log.Println("redis connected, rate limiting enabled")

	limiter := ratelimiter.New(adapter, int64(cfg.RateLimit.RPS), time.Second)
	return limiter, func() { adapter.Close() }
}

// initBroker создаёт брокер: RabbitMQ если RABBITMQ_URL задан, иначе LocalBroker.
func initBroker(cfg *config.Config) (broker.Broker, func()) {
	if cfg.RabbitMQ.URL == "" {
		log.Println("RABBITMQ_URL not set, using local worker pool")
		return broker.NewLocalBroker(cfg.Worker.PoolSize, cfg.Worker.QueueSize), nil
	}

	rmq, err := broker.NewRabbitMQBroker(cfg.RabbitMQ.URL)
	if err != nil {
		log.Fatalf("rabbitmq: %v", err)
	}
	log.Println("rabbitmq connected, durable queue with DLQ")

	return rmq, func() { rmq.Shutdown() }
}
