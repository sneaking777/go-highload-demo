// Package main — точка входа Notification Service.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-highload-demo/internal/app"
	"github.com/go-highload-demo/internal/config"
	"github.com/go-highload-demo/internal/repository"
	"github.com/go-highload-demo/internal/storage"
)

// noopLimiter — заглушка rate limiter до подключения Redis.
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
	store, closeFn := initStore(ctx, cfg)
	if closeFn != nil {
		defer closeFn()
	}

	repo := repository.New(store)
	limiter := &noopLimiter{} // TODO: заменить на ratelimiter.New(redisClient, ...)

	a := app.New(cfg, repo, limiter)
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
