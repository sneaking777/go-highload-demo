// Package service реализует бизнес-логику сервиса уведомлений.
package service

import (
	"context"
	"fmt"

	"github.com/go-highload-demo/internal/model"
	"github.com/go-highload-demo/internal/worker"
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
// сохранение в репозиторий, отправку задачи в worker pool,
// асинхронную проверку лимитов, отправку и обновление статуса.
type NotificationService struct {
	repo    Repository
	limiter RateLimiter
	senders map[model.Channel]Sender
	pool    *worker.Pool
}

// New создаёт новый NotificationService с указанными зависимостями и worker pool.
func New(repo Repository, limiter RateLimiter, poolSize, queueSize int) *NotificationService {
	svc := &NotificationService{
		repo:    repo,
		limiter: limiter,
		senders: make(map[model.Channel]Sender),
	}
	svc.pool = worker.New(poolSize, queueSize, svc.processJob)
	return svc
}

// Start запускает worker pool для асинхронной обработки уведомлений.
func (s *NotificationService) Start(ctx context.Context) {
	s.pool.Start(ctx)
}

// Stop выполняет graceful shutdown worker pool, дожидаясь завершения всех задач.
func (s *NotificationService) Stop() {
	s.pool.Stop()
}

// RegisterSender регистрирует отправщик для указанного канала доставки.
func (s *NotificationService) RegisterSender(ch model.Channel, sndr Sender) {
	s.senders[ch] = sndr
}

// Send сохраняет уведомление и отправляет задачу в worker pool для асинхронной обработки.
func (s *NotificationService) Send(ctx context.Context, n *model.Notification) error {
	if err := s.repo.Save(ctx, n); err != nil {
		return fmt.Errorf("service: save failed: %w", err)
	}

	job := worker.Job{ID: n.ID, Payload: n}
	if err := s.pool.Submit(ctx, job); err != nil {
		return fmt.Errorf("service: submit failed: %w", err)
	}

	return nil
}

// processJob — обработчик задач worker pool.
// Выполняет проверку rate limit, отправку и обновление статуса.
func (s *NotificationService) processJob(ctx context.Context, job worker.Job) error {
	n := job.Payload.(*model.Notification)

	allowed, err := s.limiter.Allow(ctx, n.UserID)
	if err != nil {
		_ = s.repo.UpdateStatus(ctx, n.ID, model.StatusFailed, err.Error())
		return nil
	}
	if !allowed {
		_ = s.repo.UpdateStatus(ctx, n.ID, model.StatusFailed, "rate limit exceeded")
		return nil
	}

	sndr, ok := s.senders[n.Channel]
	if !ok {
		_ = s.repo.UpdateStatus(ctx, n.ID, model.StatusFailed, "sender not found for channel "+string(n.Channel))
		return nil
	}

	if err := sndr.Send(ctx, n); err != nil {
		_ = s.repo.UpdateStatus(ctx, n.ID, model.StatusFailed, err.Error())
		return nil
	}

	_ = s.repo.UpdateStatus(ctx, n.ID, model.StatusSent, "")
	return nil
}

// GetByID возвращает уведомление по идентификатору.
func (s *NotificationService) GetByID(ctx context.Context, id string) (*model.Notification, error) {
	n, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service: get by id failed: %w", err)
	}
	return n, nil
}
