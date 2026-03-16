package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-highload-demo/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepository имитирует репозиторий уведомлений для тестов.
// Потокобезопасен для использования из воркеров.
type mockRepository struct {
	mu            sync.Mutex
	notifications map[string]*model.Notification
	saveErr       error
}

func newMockRepository() *mockRepository {
	return &mockRepository{notifications: make(map[string]*model.Notification)}
}

func (m *mockRepository) Save(ctx context.Context, n *model.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.notifications[n.ID] = n
	return nil
}

func (m *mockRepository) GetByID(ctx context.Context, id string) (*model.Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.notifications[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return n, nil
}

func (m *mockRepository) UpdateStatus(ctx context.Context, id string, status model.NotificationStatus, lastError string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.notifications[id]
	if !ok {
		return errors.New("not found")
	}
	n.Status = status
	n.LastError = lastError
	return nil
}

func (m *mockRepository) getStatus(id string) model.NotificationStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n, ok := m.notifications[id]; ok {
		return n.Status
	}
	return ""
}

func (m *mockRepository) getLastError(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n, ok := m.notifications[id]; ok {
		return n.LastError
	}
	return ""
}

// mockSender имитирует отправщик уведомлений для тестов.
// Потокобезопасен для использования из воркеров.
type mockSender struct {
	mu   sync.Mutex
	sent []*model.Notification
	err  error
}

func (m *mockSender) Send(ctx context.Context, n *model.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, n)
	return nil
}

func (m *mockSender) sentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

// mockRateLimiter имитирует rate limiter для тестов.
type mockRateLimiter struct {
	allowed bool
	err     error
}

func (m *mockRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return m.allowed, m.err
}

// --- Тесты Send (синхронная часть: save + submit) ---

// TestSend_SavesAndSubmits проверяет, что Send сохраняет уведомление и кладёт задачу в очередь.
func TestSend_SavesAndSubmits(t *testing.T) {
	repo := newMockRepository()
	sndr := &mockSender{}
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter, 1, 10)
	svc.RegisterSender(model.ChannelEmail, sndr)
	// Не запускаем pool — задачи остаются в очереди

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	err := svc.Send(context.Background(), n)

	require.NoError(t, err)
	assert.Equal(t, model.StatusPending, repo.getStatus(n.ID))
}

// TestSend_SaveError проверяет проброс ошибки при сбое сохранения.
func TestSend_SaveError(t *testing.T) {
	repo := newMockRepository()
	repo.saveErr = errors.New("db down")
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter, 1, 10)

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	err := svc.Send(context.Background(), n)

	assert.Error(t, err)
}

// --- Тесты Process (асинхронная обработка воркером) ---

// TestProcess_Success проверяет, что воркер обрабатывает задачу: отправляет и ставит статус sent.
func TestProcess_Success(t *testing.T) {
	repo := newMockRepository()
	sndr := &mockSender{}
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter, 1, 10)
	svc.RegisterSender(model.ChannelEmail, sndr)

	ctx := context.Background()
	svc.Start(ctx)
	defer svc.Stop()

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	require.NoError(t, svc.Send(ctx, n))

	assert.Eventually(t, func() bool {
		return repo.getStatus(n.ID) == model.StatusSent
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, 1, sndr.sentCount())
}

// TestProcess_RateLimited проверяет, что воркер ставит статус failed при превышении лимита.
func TestProcess_RateLimited(t *testing.T) {
	repo := newMockRepository()
	limiter := &mockRateLimiter{allowed: false}
	svc := New(repo, limiter, 1, 10)
	svc.RegisterSender(model.ChannelEmail, &mockSender{})

	ctx := context.Background()
	svc.Start(ctx)
	defer svc.Stop()

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	_ = svc.Send(ctx, n)

	assert.Eventually(t, func() bool {
		return repo.getStatus(n.ID) == model.StatusFailed
	}, time.Second, 10*time.Millisecond)
}

// TestProcess_SenderNotFound проверяет, что воркер ставит failed при отсутствии отправщика.
func TestProcess_SenderNotFound(t *testing.T) {
	repo := newMockRepository()
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter, 1, 10)
	// не регистрируем sender

	ctx := context.Background()
	svc.Start(ctx)
	defer svc.Stop()

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	_ = svc.Send(ctx, n)

	assert.Eventually(t, func() bool {
		return repo.getStatus(n.ID) == model.StatusFailed
	}, time.Second, 10*time.Millisecond)
}

// TestProcess_SenderError проверяет, что при ошибке отправки воркер ставит failed и сохраняет ошибку.
func TestProcess_SenderError(t *testing.T) {
	repo := newMockRepository()
	sndr := &mockSender{err: errors.New("connection timeout")}
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter, 1, 10)
	svc.RegisterSender(model.ChannelPush, sndr)

	ctx := context.Background()
	svc.Start(ctx)
	defer svc.Stop()

	n := model.NewNotification("user:1", model.ChannelPush, "hello")
	_ = svc.Send(ctx, n)

	assert.Eventually(t, func() bool {
		return repo.getStatus(n.ID) == model.StatusFailed
	}, time.Second, 10*time.Millisecond)
	assert.Eventually(t, func() bool {
		return repo.getLastError(n.ID) == "connection timeout"
	}, time.Second, 10*time.Millisecond)
}

// TestProcess_RateLimiterError проверяет, что при ошибке rate limiter воркер ставит failed.
func TestProcess_RateLimiterError(t *testing.T) {
	repo := newMockRepository()
	limiter := &mockRateLimiter{err: errors.New("redis down")}
	svc := New(repo, limiter, 1, 10)
	svc.RegisterSender(model.ChannelEmail, &mockSender{})

	ctx := context.Background()
	svc.Start(ctx)
	defer svc.Stop()

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	_ = svc.Send(ctx, n)

	assert.Eventually(t, func() bool {
		return repo.getStatus(n.ID) == model.StatusFailed
	}, time.Second, 10*time.Millisecond)
}

// --- Тесты GetByID ---

// TestGetByID_Success проверяет получение уведомления по ID.
func TestGetByID_Success(t *testing.T) {
	repo := newMockRepository()
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter, 1, 10)
	svc.RegisterSender(model.ChannelEmail, &mockSender{})

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	_ = svc.Send(context.Background(), n)

	found, err := svc.GetByID(context.Background(), n.ID)
	require.NoError(t, err)
	assert.Equal(t, n.ID, found.ID)
}

// TestGetByID_NotFound проверяет ошибку при запросе несуществующего уведомления.
func TestGetByID_NotFound(t *testing.T) {
	repo := newMockRepository()
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter, 1, 10)

	_, err := svc.GetByID(context.Background(), "nonexistent")
	assert.Error(t, err)
}
