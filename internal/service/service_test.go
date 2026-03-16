package service

import (
	"context"
	"errors"
	"sync"
	"testing"

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

// mockPublisher имитирует брокер для тестов.
type mockPublisher struct {
	published []*model.Notification
	err       error
}

func (m *mockPublisher) Publish(ctx context.Context, n *model.Notification) error {
	if m.err != nil {
		return m.err
	}
	m.published = append(m.published, n)
	return nil
}

// --- Тесты Send (save + publish) ---

// TestSend_SavesAndPublishes проверяет, что Send сохраняет уведомление и публикует в очередь.
func TestSend_SavesAndPublishes(t *testing.T) {
	repo := newMockRepository()
	pub := &mockPublisher{}
	svc := New(repo, &mockRateLimiter{allowed: true}, pub)

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	err := svc.Send(context.Background(), n)

	require.NoError(t, err)
	assert.Equal(t, model.StatusPending, repo.getStatus(n.ID))
	assert.Len(t, pub.published, 1)
}

// TestSend_SaveError проверяет проброс ошибки при сбое сохранения.
func TestSend_SaveError(t *testing.T) {
	repo := newMockRepository()
	repo.saveErr = errors.New("db down")
	pub := &mockPublisher{}
	svc := New(repo, &mockRateLimiter{allowed: true}, pub)

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	err := svc.Send(context.Background(), n)

	assert.Error(t, err)
	assert.Empty(t, pub.published)
}

// TestSend_PublishError проверяет проброс ошибки при сбое публикации.
func TestSend_PublishError(t *testing.T) {
	repo := newMockRepository()
	pub := &mockPublisher{err: errors.New("broker down")}
	svc := New(repo, &mockRateLimiter{allowed: true}, pub)

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	err := svc.Send(context.Background(), n)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "publish")
}

// --- Тесты ProcessNotification (синхронная обработка) ---

// TestProcess_Success проверяет успешную обработку: отправка + статус sent.
func TestProcess_Success(t *testing.T) {
	repo := newMockRepository()
	sndr := &mockSender{}
	svc := New(repo, &mockRateLimiter{allowed: true}, &mockPublisher{})
	svc.RegisterSender(model.ChannelEmail, sndr)

	ctx := context.Background()
	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	_ = repo.Save(ctx, n)

	svc.ProcessNotification(ctx, n)

	assert.Equal(t, model.StatusSent, repo.getStatus(n.ID))
	assert.Equal(t, 1, sndr.sentCount())
}

// TestProcess_RateLimited проверяет, что при превышении лимита статус — failed.
func TestProcess_RateLimited(t *testing.T) {
	repo := newMockRepository()
	svc := New(repo, &mockRateLimiter{allowed: false}, &mockPublisher{})
	svc.RegisterSender(model.ChannelEmail, &mockSender{})

	ctx := context.Background()
	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	_ = repo.Save(ctx, n)

	svc.ProcessNotification(ctx, n)

	assert.Equal(t, model.StatusFailed, repo.getStatus(n.ID))
}

// TestProcess_SenderNotFound проверяет ошибку при отсутствии отправщика.
func TestProcess_SenderNotFound(t *testing.T) {
	repo := newMockRepository()
	svc := New(repo, &mockRateLimiter{allowed: true}, &mockPublisher{})
	// не регистрируем sender

	ctx := context.Background()
	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	_ = repo.Save(ctx, n)

	svc.ProcessNotification(ctx, n)

	assert.Equal(t, model.StatusFailed, repo.getStatus(n.ID))
}

// TestProcess_SenderError проверяет, что при ошибке отправки статус — failed с сообщением.
func TestProcess_SenderError(t *testing.T) {
	repo := newMockRepository()
	sndr := &mockSender{err: errors.New("connection timeout")}
	svc := New(repo, &mockRateLimiter{allowed: true}, &mockPublisher{})
	svc.RegisterSender(model.ChannelPush, sndr)

	ctx := context.Background()
	n := model.NewNotification("user:1", model.ChannelPush, "hello")
	_ = repo.Save(ctx, n)

	svc.ProcessNotification(ctx, n)

	assert.Equal(t, model.StatusFailed, repo.getStatus(n.ID))
	assert.Equal(t, "connection timeout", repo.getLastError(n.ID))
}

// TestProcess_RateLimiterError проверяет, что при ошибке rate limiter статус — failed.
func TestProcess_RateLimiterError(t *testing.T) {
	repo := newMockRepository()
	svc := New(repo, &mockRateLimiter{err: errors.New("redis down")}, &mockPublisher{})
	svc.RegisterSender(model.ChannelEmail, &mockSender{})

	ctx := context.Background()
	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	_ = repo.Save(ctx, n)

	svc.ProcessNotification(ctx, n)

	assert.Equal(t, model.StatusFailed, repo.getStatus(n.ID))
}

// --- Тесты GetByID ---

// TestGetByID_Success проверяет получение уведомления по ID.
func TestGetByID_Success(t *testing.T) {
	repo := newMockRepository()
	svc := New(repo, &mockRateLimiter{allowed: true}, &mockPublisher{})

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	_ = svc.Send(context.Background(), n)

	found, err := svc.GetByID(context.Background(), n.ID)
	require.NoError(t, err)
	assert.Equal(t, n.ID, found.ID)
}

// TestGetByID_NotFound проверяет ошибку при запросе несуществующего уведомления.
func TestGetByID_NotFound(t *testing.T) {
	repo := newMockRepository()
	svc := New(repo, &mockRateLimiter{allowed: true}, &mockPublisher{})

	_, err := svc.GetByID(context.Background(), "nonexistent")
	assert.Error(t, err)
}
