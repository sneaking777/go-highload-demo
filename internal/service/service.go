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

// NotificationService координирует отправку уведомлений:
// сохранение, проверку лимитов, отправку и обновление статуса.
type NotificationService struct {
	repo Repository
	limiter RateLimiter
	senders map[model.Channel]Sender
}

// New создаёт новый NotificationService с указанными зависимостями.
func New(repo Repository, limiter RateLimiter) *NotificationService  {
	return &NotificationService{
		repo: repo,
		limiter: limiter,
		senders: make(map[model.Channel]Sender),
	}	
}

// RegisterSender регистрирует отправщик для указанного канала доставки.
func (s *NotificationService) RegisterSender(ch model.Channel, sndr Sender)  {
	s.senders[ch] = sndr	
}

// Send выполняет полный цикл отправки уведомления:
// сохранение → проверка rate limit → отправка → обновление статуса.
func (s *NotificationService) Send(ctx context.Context, n *model.Notification) error  {
	if err := s.repo.Save(ctx, n); err != nil {
		return fmt.Errorf("service: save failed: %w", err)		
	}
	allowed, err := s.limiter.Allow(ctx, n.UserID)
	if err != nil {
		return fmt.Errorf("service: rate Limiter failed: %w", err)	
	}
	if !allowed {
		return fmt.Errorf("service: rate limit exceeded for user %s", n.UserID)	
	}
	
	sndr, ok := s.senders[n.Channel]
	if !ok {
		return fmt.Errorf("service: sender not found for channel %s", n.Channel)	
	}

	if err := sndr.Send(ctx, n); err != nil {
		_ = s.repo.UpdateStatus(ctx, n.ID, model.StatusFailed, err.Error())
		return fmt.Errorf("service: send failed: %w", err)	
	}

	if err := s.repo.UpdateStatus(ctx, n.ID, model.StatusSent, ""); err != nil {
		return fmt.Errorf("service: update status failed: %w", err)
	}

	return nil
}

// GetByID возвращает уведомление по идентификатору.
func (s *NotificationService) GetByID(ctx context.Context, id string) (*model.Notification, error)  {
	n, err := s.repo.GetByID(ctx, id)
	if err !=nil {
		return nil, fmt.Errorf("service: get by id failed: %w", err)	
	}
	return n, nil	
}