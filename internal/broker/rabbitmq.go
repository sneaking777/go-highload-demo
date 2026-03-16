package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/go-highload-demo/internal/model"
)

const (
	exchangeName = "notifications"
	queueName    = "notifications.process"
	dlqName      = "notifications.dlq"
	routingKey   = "notification.new"
)

// RabbitMQBroker реализует Broker через RabbitMQ с поддержкой
// durable queue, dead letter queue и persistent messages.
type RabbitMQBroker struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	done chan struct{}
}

// NewRabbitMQBroker подключается к RabbitMQ и объявляет exchange,
// основную очередь с DLQ и binding.
func NewRabbitMQBroker(url string) (*RabbitMQBroker, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: dial failed: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: channel failed: %w", err)
	}

	// Direct exchange для маршрутизации уведомлений
	if err := ch.ExchangeDeclare(exchangeName, "direct", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: exchange declare failed: %w", err)
	}

	// Dead letter queue для необработанных сообщений
	if _, err := ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: dlq declare failed: %w", err)
	}

	// Основная очередь с перенаправлением в DLQ при отказе
	args := amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": dlqName,
	}
	if _, err := ch.QueueDeclare(queueName, true, false, false, false, args); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: queue declare failed: %w", err)
	}

	if err := ch.QueueBind(queueName, routingKey, exchangeName, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: bind failed: %w", err)
	}

	return &RabbitMQBroker{conn: conn, ch: ch, done: make(chan struct{})}, nil
}

// Publish сериализует уведомление в JSON и отправляет в RabbitMQ
// как persistent message.
func (b *RabbitMQBroker) Publish(ctx context.Context, n *model.Notification) error {
	body, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("rabbitmq: marshal failed: %w", err)
	}

	return b.ch.PublishWithContext(ctx, exchangeName, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

// Run запускает consumer, который читает сообщения из очереди
// и передаёт их обработчику. Prefetch ограничивает количество
// одновременно обрабатываемых сообщений.
func (b *RabbitMQBroker) Run(ctx context.Context, handler Handler) error {
	if err := b.ch.Qos(10, 0, false); err != nil {
		return fmt.Errorf("rabbitmq: qos failed: %w", err)
	}

	msgs, err := b.ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("rabbitmq: consume failed: %w", err)
	}

	go func() {
		defer close(b.done)
		for msg := range msgs {
			var n model.Notification
			if err := json.Unmarshal(msg.Body, &n); err != nil {
				log.Printf("rabbitmq: unmarshal failed: %v", err)
				msg.Nack(false, false) // отправить в DLQ
				continue
			}
			handler(ctx, &n)
			msg.Ack(false)
		}
	}()

	return nil
}

// Shutdown закрывает канал и соединение с RabbitMQ.
func (b *RabbitMQBroker) Shutdown() error {
	if err := b.ch.Close(); err != nil {
		b.conn.Close()
		return err
	}
	return b.conn.Close()
}
