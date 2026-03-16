package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoad_Defaults проверяет, что Load возвращает корректные значения по умолчанию без переменных окружения.
func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load()
	require.NoError(t, err)
	
	assert.Equal(t, ":8080", cfg.Server.Addr)
	assert.Equal(t, 10, cfg.Worker.PoolSize)
	assert.Equal(t, 100, cfg.Worker.QueueSize)
	assert.Equal(t, 100, cfg.RateLimit.RPS)
	assert.Equal(t, 3, cfg.Retry.MaxAttempts)
}

// TestLoad_FromEnv проверяет, что Load корректно читает все параметры из переменных окружения.
func TestLoad_FromEnv(t *testing.T) {
	envs := map[string]string{
		"SERVER_ADDR":        ":9090",
		"DB_HOST":            "db",
		"DB_PORT":            "5432",
		"DB_USER":            "user",
		"DB_PASSWORD":        "pass",
		"DB_NAME":            "test",
		"RABBITMQ_URL":       "amqp://guest:guest@rabbitmq:5672/",
		"REDIS_ADDR":         "redis:6379",
		"WORKER_POOL_SIZE":   "20",
		"WORKER_QUEUE_SIZE":  "200",
		"RATE_LIMIT_RPS":     "50",
		"RETRY_MAX_ATTEMPTS": "5",
	}
	
	for k, v := range envs {
		os.Setenv(k, v)
	}

	t.Cleanup(func() {
		for k := range envs {
			os.Unsetenv(k)
		}
	})

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, ":9090", cfg.Server.Addr)
	assert.Equal(t, "postgres://user:pass@db:5432/test?sslmode=disable", cfg.Postgres.DSN())
	assert.Equal(t, "amqp://guest:guest@rabbitmq:5672/", cfg.RabbitMQ.URL)
	assert.Equal(t, "redis:6379", cfg.Redis.Addr)
	assert.Equal(t, 20, cfg.Worker.PoolSize)
	assert.Equal(t, 200, cfg.Worker.QueueSize)
	assert.Equal(t, 50, cfg.RateLimit.RPS)
	assert.Equal(t, 5, cfg.Retry.MaxAttempts)
}

// TestLoad_InvalidPoolSize проверяет, что Load возвращает ошибку при невалидном числовом значении.
func TestLoad_InvalidPoolSize(t *testing.T) {
	os.Setenv("WORKER_POOL_SIZE", "not_a_number")
	t.Cleanup(func() {os.Unsetenv("WORKER_POOL_SIZE")})

	_, err := Load()
	assert.Error(t, err)
}