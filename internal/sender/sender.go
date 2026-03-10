package sender

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/go-highload-demo/internal/model"
)

// Sender определяет интерфейс отправщика уведомлений.
type Sender interface {
	// Send отправляет уведомление. Возвращает ошибку при сбое или отмене контекста.
	Send(ctx context.Context, n *model.Notification) error
}

// baseSender содержит общую логику имитации отправки с сетевой задержкой.
type baseSender struct {
	channel model.Channel
}

// send имитирует отправку с задержкой 10-50mc и проверкой контекста
func (b *baseSender) send(ctx context.Context, n *model.Notification) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("%s: контекст отменён: %w", b.channel, ctx.Err())
	default:
	}

	delay := time.Duration(10+rand.Intn(41)) * time.Millisecond
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%s: контекст отменён: %w", b.channel, ctx.Err())
	}
}

// EmailSender отправляет уведомления по электронной почте.
type EmailSender struct {
	baseSender
}

// NewEmailSender создаёт новый EmailSender.
func NewEmailSender() *EmailSender {
	return &EmailSender{baseSender{channel: model.ChannelEmail}}
}

// Send отправляет email-уведомление.
func (s *EmailSender) Send(ctx context.Context, n *model.Notification) error {
	return s.send(ctx, n)
}

// PushSender отправляет push-уведомления.
type PushSender struct {
	baseSender
}

// NewPushSender создаёт новый PushSender.
func NewPushSender() *PushSender {
	return &PushSender{baseSender{channel: model.ChannelPush}}
}

// Send отправляет push-уведомление.
func (s *PushSender) Send(ctx context.Context, n *model.Notification) error {
	return s.send(ctx, n)
}

// SMSSender отправляет SMS-уведомления.
type SMSSender struct {
	baseSender
}

// NewSMSSender создаёт новый SMSSender.
func NewSMSSender() *SMSSender {
	return &SMSSender{baseSender{channel: model.ChannelSMS}}
}

// Send отправляет SMS-уведомление.
func (s *SMSSender) Send(ctx context.Context, n *model.Notification) error {
	return s.send(ctx, n)
}

// WebhookSender отправляет уведомления через webhook.
type WebhookSender struct {
	baseSender
}

// NewWebhookSender создаёт новый WebhookSender.
func NewWebhookSender() *WebhookSender {
	return &WebhookSender{baseSender{channel: model.ChannelWebhook}}
}

// Send отправляет webhook-уведомление.
func (s *WebhookSender) Send(ctx context.Context, n *model.Notification) error {
	return s.send(ctx, n)
}

// Registry хранит отправщиков по каналам доставки и обеспечивает потокобезопасный доступ.
type Registry struct {
	mu      sync.RWMutex
	senders map[model.Channel]Sender
}

// NewRegistry создаёт новый пустой реестр отправщиков.
func NewRegistry() *Registry {
	return &Registry{
		senders: make(map[model.Channel]Sender),
	}
}

// Register добавляет отправщик для указанного канала.
func (r *Registry) Register(ch model.Channel, s Sender) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.senders[ch] = s
}

// Get возвращает отправщик по каналу. Второе значение — найден ли отправщик.
func (r *Registry) Get(ch model.Channel) (Sender, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.senders[ch]
	return s, ok
}
