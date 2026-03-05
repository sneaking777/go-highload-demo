package worker

import (
	"context"
	"errors"
	"sync"
)

// Job представляет единицу работы для обработки воркером.
type Job struct {
	ID      string
	Payload any
}

// Handler — функция обработки задачи воркером.
type Handler func(ctx context.Context, job Job) error

// Pool управляет пулом воркеров, обрабатывающих задачи из буферизированной очереди.
type Pool struct {
	size    int
	queue   chan Job
	handler Handler
	wg      sync.WaitGroup
	cancel  context.CancelFunc
	stopped chan struct{}
}

// New создаёт новый пул с указанным количеством воркеров и размером очереди.
func New(size, queueSize int, handler Handler) *Pool {
	return &Pool{
		size:    size,
		queue:   make(chan Job, queueSize),
		handler: handler,
		stopped: make(chan struct{}),
	}
}

// Start запускает воркеры пула. Каждый воркер читает задачи из очереди до её закрытия.
func (p *Pool) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)

	for i := 0; i < p.size; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for job := range p.queue {
				p.handler(ctx, job)
			}
		}()
	}
}

// Stop выполняет graceful shutdown: закрывает очередь и ожидает завершения всех воркеров.
func (p *Pool) Stop() {
	close(p.queue)
	p.wg.Wait()
	p.cancel()
	close(p.stopped)
}

// Submit отправляет задачу в очередь. Возвращает ошибку если пул остановлен или контекст отменён.
func (p *Pool) Submit(ctx context.Context, job Job) error {
	select {
	case <-p.stopped:
		return errors.New("worker pool is stopped")
	default:
	}

	select {
	case p.queue <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
