// Package service реализует бизнес-логику сервиса уведомлений.
package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-highload-demo/internal/model"
	"github.com/go-highload-demo/pkg/retry"
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
// асинхронную проверку лимитов, отправку с retry и обновление статуса.
type NotificationService struct {
	repo     Repository
	limiter  RateLimiter
	senders  map[model.Channel]Sender
	pub      Publisher
	retryCfg retry.Config
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

// SetRetryConfig устанавливает конфигурацию повторных попыток отправки.
// Если не вызван, отправка выполняется однократно.
func (s *NotificationService) SetRetryConfig(cfg retry.Config) {
	s.retryCfg = cfg
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

// SendAll выполняет fan-out: создаёт уведомление для каждого канала
// и отправляет их параллельно. Возвращает список ID созданных уведомлений.
func (s *NotificationService) SendAll(ctx context.Context, userID string, channels []model.Channel, payload string) ([]string, error) {
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		ids      []string
		firstErr error
	)

	for _, ch := range channels {
		wg.Add(1)
		go func(ch model.Channel) {
			defer wg.Done()
			n := model.NewNotification(userID, ch, payload)
			if err := s.Send(ctx, n); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			ids = append(ids, n.ID)
			mu.Unlock()
		}(ch)
	}

	wg.Wait()
	return ids, firstErr
}

// ProcessNotification обрабатывает уведомление из очереди:
// проверка rate limit → отправка через sender с retry → обновление статуса.
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

	sendFn := func(ctx context.Context) error {
		return sndr.Send(ctx, n)
	}

	if s.retryCfg.MaxAttempts > 1 {
		err = retry.Do(ctx, s.retryCfg, sendFn)
	} else {
		err = sendFn(ctx)
	}

	if err != nil {
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
