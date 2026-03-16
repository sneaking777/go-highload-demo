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
func (n *noopLimiter) Allow(_ context.Context, _ string) (bool, error)  {
	return true, nil
}

// main инициализирует и запускает HTTP-сервер сервиса уведомлений.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)	
	}

	store := storage.NewMemoryStore()
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
	if err := a.Shutdown(context.Background()); err != nil {
		log.Fatalf("shutdown: %v", err)	
	}
	log.Println("server stopped")
}
