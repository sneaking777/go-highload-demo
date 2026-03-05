// Package worker реализует пул воркеров для параллельной обработки задач.
package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPool_ProcessesJobs проверяет, что пул обрабатывает все отправленные задачи.
func TestPool_ProcessesJobs(t *testing.T) {
	var processed atomic.Int32

	p := New(4, 10, func(ctx context.Context, job Job) error {
		processed.Add(1)
		return nil
	})

	ctx := context.Background()
	p.Start(ctx)

	for i := 0; i < 10; i++ {
		err := p.Submit(ctx, Job{ID: string(rune('A' + i))})
		require.NoError(t, err)
	}

	p.Stop()
	assert.Equal(t, int32(10), processed.Load())
}

// TestPool_ConcurrentWorkers проверяет, что задачи выполняются параллельно несколькими воркерами.
func TestPool_ConcurrentWorkers(t *testing.T) {
	var peak atomic.Int32
	var current atomic.Int32

	p := New(4, 10, func(ctx context.Context, job Job) error {
		val := current.Add(1)
		for {
			old := peak.Load()
			if val <= old || peak.CompareAndSwap(old, val) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		current.Add(-1)
		return nil
	})

	ctx := context.Background()
	p.Start(ctx)

	for i := 0; i < 8; i++ {
		_ = p.Submit(ctx, Job{ID: "job"})
	}

	p.Stop()
	assert.GreaterOrEqual(t, peak.Load(), int32(2), "должно быть минимум 2 параллельных воркера")
}

// TestPool_GracefulShutdown проверяет, что Stop дожидается завершения текущих задач.
func TestPool_GracefulShutdown(t *testing.T) {
	var processed atomic.Int32

	p := New(2, 5, func(ctx context.Context, job Job) error {
		time.Sleep(50 * time.Millisecond)
		processed.Add(1)
		return nil
	})

	ctx := context.Background()
	p.Start(ctx)

	for i := 0; i < 5; i++ {
		_ = p.Submit(ctx, Job{ID: "job"})
	}

	p.Stop()
	assert.Equal(t, int32(5), processed.Load(), "все задачи должны быть обработаны до завершения")
}

// TestPool_SubmitAfterStop проверяет, что Submit возвращает ошибку после остановки пула.
func TestPool_SubmitAfterStop(t *testing.T) {
	p := New(2, 5, func(ctx context.Context, job Job) error {
		return nil
	})

	ctx := context.Background()
	p.Start(ctx)
	p.Stop()

	err := p.Submit(ctx, Job{ID: "late"})
	assert.Error(t, err)
}

// TestPool_SubmitContextCancelled проверяет, что Submit возвращает ошибку при отменённом контексте.
func TestPool_SubmitContextCancelled(t *testing.T) {
	p := New(1, 1, func(ctx context.Context, job Job) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	ctx := context.Background()
	p.Start(ctx)

	// Заполняем очередь и воркер
	_ = p.Submit(ctx, Job{ID: "fill-worker"})
	_ = p.Submit(ctx, Job{ID: "fill-queue"})

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	err := p.Submit(cancelCtx, Job{ID: "blocked"})
	assert.Error(t, err)

	p.Stop()
}
