package broker

import (
	"context"

	"github.com/go-highload-demo/internal/model"
	"github.com/go-highload-demo/internal/worker"
)

// LocalBroker реализует Broker через in-memory worker pool.
// Используется при отсутствии RabbitMQ.
type LocalBroker struct {
	pool    *worker.Pool
	handler Handler
}

// NewLocalBroker создаёт локальный брокер с worker pool заданного размера.
func NewLocalBroker(poolSize, queueSize int) *LocalBroker {
	b := &LocalBroker{}
	b.pool = worker.New(poolSize, queueSize, func(ctx context.Context, job worker.Job) error {
		n := job.Payload.(*model.Notification)
		if b.handler != nil {
			b.handler(ctx, n)
		}
		return nil
	})
	return b
}

// Publish отправляет уведомление в worker pool.
func (b *LocalBroker) Publish(ctx context.Context, n *model.Notification) error {
	return b.pool.Submit(ctx, worker.Job{ID: n.ID, Payload: n})
}

// Run устанавливает обработчик и запускает worker pool.
func (b *LocalBroker) Run(ctx context.Context, handler Handler) error {
	b.handler = handler
	b.pool.Start(ctx)
	return nil
}

// Shutdown выполняет graceful shutdown worker pool.
func (b *LocalBroker) Shutdown() error {
	b.pool.Stop()
	return nil
}
