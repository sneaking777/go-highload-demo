// Package service реализует бизнес-логику сервиса уведомлений.
package service

import (
	"context"
	"fmt"

	"github.com/go-highload-demo/internal/model"
)

// Repository определяет интерфейс хранилища, используемого сервисом.
type Repository interface {
	Save(ctx context.Context, n *model.Notification) error
	GetByID(ctx context.Context, id string) (*model.Notification, error)
	UpdateStatus(ctx context.Context, id string, status model.NotificationStatus, lastError string) error
}

// Sender определяет интерфейс отправщика уведомлений.
type Sender interface {
	Send(ctx context.Context, n *model.Notification) error
}

// RateLimiter определяет интерфейс ограничителя частоты запросов.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// Publisher определяет интерфейс публикации уведомлений в очередь.
type Publisher interface {
	Publish(ctx context.Context, n *model.Notification) error
}

// NotificationService координирует отправку уведомлений:
// сохранение в репозиторий, публикацию в очередь (broker),
// асинхронную проверку лимитов, отправку и обновление статуса.
type NotificationService struct {
	repo    Repository
	limiter RateLimiter
	senders map[model.Channel]Sender
	pub     Publisher
}

// New создаёт новый NotificationService с указанными зависимостями.
func New(repo Repository, limiter RateLimiter, pub Publisher) *NotificationService {
	return &NotificationService{
		repo:    repo,
		limiter: limiter,
		senders: make(map[model.Channel]Sender),
		pub:     pub,
	}
}

// RegisterSender регистрирует отправщик для указанного канала доставки.
func (s *NotificationService) RegisterSender(ch model.Channel, sndr Sender) {
	s.senders[ch] = sndr
}

// Send сохраняет уведомление и публикует его в очередь для асинхронной обработки.
func (s *NotificationService) Send(ctx context.Context, n *model.Notification) error {
	if err := s.repo.Save(ctx, n); err != nil {
		return fmt.Errorf("service: save failed: %w", err)
	}

	if err := s.pub.Publish(ctx, n); err != nil {
		return fmt.Errorf("service: publish failed: %w", err)
	}

	return nil
}

// ProcessNotification обрабатывает уведомление из очереди:
// проверка rate limit → отправка через sender → обновление статуса.
func (s *NotificationService) ProcessNotification(ctx context.Context, n *model.Notification) {
	allowed, err := s.limiter.Allow(ctx, n.UserID)
	if err != nil {
		_ = s.repo.UpdateStatus(ctx, n.ID, model.StatusFailed, err.Error())
		return
	}
	if !allowed {
		_ = s.repo.UpdateStatus(ctx, n.ID, model.StatusFailed, "rate limit exceeded")
		return
	}

	sndr, ok := s.senders[n.Channel]
	if !ok {
		_ = s.repo.UpdateStatus(ctx, n.ID, model.StatusFailed, "sender not found for channel "+string(n.Channel))
		return
	}

	if err := sndr.Send(ctx, n); err != nil {
		_ = s.repo.UpdateStatus(ctx, n.ID, model.StatusFailed, err.Error())
		return
	}

	_ = s.repo.UpdateStatus(ctx, n.ID, model.StatusSent, "")
}

// GetByID возвращает уведомление по идентификатору.
func (s *NotificationService) GetByID(ctx context.Context, id string) (*model.Notification, error) {
	n, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service: get by id failed: %w", err)
	}
	return n, nil
}
