// Package retry реализует повторные попытки вызова с экспоненциальной задержкой (exponential backoff).
package retry

import (
	"context"
	"math"
	"time"
)

// Config задаёт параметры повторных попыток.
type Config struct {
	// MaxAttempts — максимальное количество попыток (включая первую).
	MaxAttempts int

	// BaseDelay — начальная задержка между попытками.
	BaseDelay time.Duration

	// MaxDelay — максимально допустимая задержка.
	MaxDelay time.Duration

	// Multiplier — коэффициент роста задержки между попытками.
	Multiplier float64
}

// Do выполняет fn с повторными попытками согласно cfg.
// Возвращает nil при успехе или последнюю ошибку при исчерпании попыток.
// Прекращает попытки при отмене контекста.
func Do(ctx context.Context, cfg Config, fn func(ctx context.Context) error) error {
	if cfg.Multiplier == 0 {
		cfg.Multiplier = 2.0
	}

	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		if attempt == cfg.MaxAttempts-1 {
			break
		}

		delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(cfg.Multiplier, float64(attempt)))
		if cfg.MaxDelay > 0 && delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}
