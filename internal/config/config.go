// Package config отвечает за загрузку конфигурации сервиса из переменных окружения.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config содержит полную конфигурацию сервиса уведомлений.
type Config struct {
	Server	ServerConfig
	Postgres	PostgresConfig
	RabbitMQ	RabbitMQConfig
	Redis		RedisConfig
	Worker		WorkerConfig
	RateLimit	RateLimitConfig
	Retry		RetryConfig
}

// ServerConfig задаёт параметры HTTP-сервера.
type ServerConfig struct {
	Addr string
}

// PostgresConfig задаёт параметры подключения к PostgreSQL.
type PostgresConfig struct {
	DSN string
}

// RabbitMQConfig задаёт параметры подключения к RabbitMQ.
type RabbitMQConfig struct {
	URL string
}

// RedisConfig задаёт параметры подключения к Redis.
type RedisConfig struct {
	Addr string
}

// WorkerConfig задаёт параметры пула воркеров.
type WorkerConfig struct {
	PoolSize int
	QueueSize int
}

// RateLimitConfig задаёт параметры ограничения частоты запросов.
type RateLimitConfig struct {
	RPS int
}

// RetryConfig задаёт параметры повторных попыток отправки.
type RetryConfig struct {
	MaxAttempts int
}

// Load загружает конфигурацию из переменных окружения, подставляя значения по умолчанию.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{Addr: envOrDefault("SERVER_ADDR", ":8080")},
		Postgres: PostgresConfig{DSN: envOrDefault("POSTGRES_DSN", "")},
		RabbitMQ: RabbitMQConfig{URL: envOrDefault("RABBITMQ_URL", "")},
		Redis: RedisConfig{Addr: envOrDefault("REDIS_ADDR", "")},
	}
	
	poolSize, err := envOrDefaultInt("WORKER_POOL_SIZE", 10)
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_POOL_SIZE: %w", err)	
	}
	queueSize, err := envOrDefaultInt("WORKER_QUEUE_SIZE", 100)
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_QUEUE_SIZE: %w", err)	
	}
	rps, err := envOrDefaultInt("RATE_LIMIT_RPS", 100)
	if err !=nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_RPS: %w", err)	
	}
	maxAttempts, err := envOrDefaultInt("RETRY_MAX_ATTEMPTS", 3)

	if err != nil {
		return nil, fmt.Errorf("invalid RETRY_MAX_ATTEMPTS: %w", err)	
	}

	cfg.Worker = WorkerConfig{PoolSize: poolSize, QueueSize: queueSize}
	cfg.RateLimit = RateLimitConfig{RPS: rps}
	cfg.Retry = RetryConfig{MaxAttempts: maxAttempts}

	return cfg, nil
}

func envOrDefault(key, defaultVal string) string  {
	if v:= os.Getenv(key); v != "" {
		return v
	}
	return defaultVal	
}

func envOrDefaultInt(key string, defaultVal int) (int, error)  {
	v := os.Getenv(key)
	if v == "" {
		return  defaultVal, nil
	}
	return strconv.Atoi(v)	
}