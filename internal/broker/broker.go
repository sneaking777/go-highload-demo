// Package broker определяет интерфейсы и реализации очередей сообщений
// для асинхронной обработки уведомлений.
package broker

import (
	"context"

	"github.com/go-highload-demo/internal/model"
)

// Handler — функция обработки уведомления из очереди.
type Handler func(ctx context.Context, n *model.Notification)

// Broker определяет интерфейс брокера сообщений.
// Реализуется LocalBroker (in-memory worker pool) и RabbitMQBroker.
type Broker interface {
	// Publish отправляет уведомление в очередь.
	Publish(ctx context.Context, n *model.Notification) error
	// Run запускает обработку сообщений с указанным обработчиком.
	Run(ctx context.Context, handler Handler) error
	// Shutdown останавливает обработку и освобождает ресурсы.
	Shutdown() error
}
