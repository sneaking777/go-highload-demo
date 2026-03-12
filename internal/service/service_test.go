package service

import (
	"context"
	"errors"
	"testing"

	"github.com/go-highload-demo/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepository имитирует репозиторий уведомлений для тестов.
type mockRepository struct {
	notifications map[string]*model.Notification
	err error
}

// newMockRepository создаёт новый mockRepository с пустым хранилищем.
func newMockRepository() *mockRepository  {
	return &mockRepository{notifications: make(map[string]*model.Notification) }	
}

func (m *mockRepository) Save(ctx context.Context, n *model.Notification) error  {
	if m.err != nil {
		return m.err	
	}
	m.notifications[n.ID] = n
	return nil
}

func (m *mockRepository) GetByID(ctx context.Context, id string) (*model.Notification, error)  {
	if m.err != nil {
		return nil, m.err	
	}
	n, ok := m.notifications[id]
	if !ok {
		return nil, errors.New("not found")	
	}
	return n, nil	
}

func (m *mockRepository) UpdateStatus(ctx context.Context, id string, status model.NotificationStatus, lastError string) error  {
	if m.err != nil {
		return m.err	
	}
	n, ok := m.notifications[id]
	if !ok {
		return errors.New("not found")	
	}
	n.Status = status
	n.LastError = lastError
	return  nil	
}

// mockSender имитирует отправщик уведомлений для тестов.
type mockSender struct {
	sent []*model.Notification
	err error
}

func (m *mockSender) Send(ctx context.Context, n *model.Notification) error  {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, n)
	return nil	
}

// mockRateLimiter имитирует rate limiter для тестов.
type mockRateLimiter struct {
	allowed bool
	err error
}

func (m *mockRateLimiter) Allow(ctx context.Context, key string) (bool, error)  {
	return m.allowed, m.err	
}

// TestSend_Success проверяет успешную отправку уведомления: сохранение, отправка, обновление статуса.
func TestSend_Success(t *testing.T)  {
	repo := newMockRepository()
	sndr := &mockSender{}
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter)
	svc.RegisterSender(model.ChannelEmail, sndr)

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	err := svc.Send(context.Background(), n)

	require.NoError(t, err)
	assert.Equal(t, model.StatusSent,repo.notifications[n.ID].Status)
	assert.Len(t, sndr.sent, 1)
}

// TestSend_RateLimited проверяет, что уведомление отклоняется при превышении лимита.
func TestSend_RateLimited(t *testing.T)  {
	repo := newMockRepository()
	sndr := &mockSender{}
	limiter := &mockRateLimiter{allowed: false}	
	svc := New(repo, limiter)
	svc.RegisterSender(model.ChannelEmail, sndr)

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	err := svc.Send(context.Background(), n)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")
	assert.Empty(t, sndr.sent)
}

// TestSend_SenderNotFound проверяет ошибку при отсутствии отправщика для канала.
func TestSend_SenderNotFound(t *testing.T)  {
	repo := newMockRepository()
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter)
	// не регистрируем sender для email

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	err := svc.Send(context.Background(), n)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sender not found")
}

// TestSend_SaveError проверяет проброс ошибки при сбое сохранения.
func TestSend_SaveError(t *testing.T)  {
	repo := newMockRepository()
	repo.err = errors.New("db down")
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter)
	svc.RegisterSender(model.ChannelEmail, &mockSender{})
	
	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	err := svc.Send(context.Background(), n)

	assert.Error(t, err)
}

// TestSend_SenderError проверяет, что при ошибке отправки статус меняется на failed.
func TestSend_SenderError(t *testing.T)  {
	repo := newMockRepository()
	sndr := &mockSender{err: errors.New("connection timeout")}
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter)
	svc.RegisterSender(model.ChannelPush, sndr)
	
	n := model.NewNotification("user:1", model.ChannelPush, "hello")
	err := svc.Send(context.Background(), n)

	assert.Error(t, err)
	assert.Equal(t, model.StatusFailed, repo.notifications[n.ID].Status)
	assert.Equal(t, "connection timeout", repo.notifications[n.ID].LastError)
}

// TestSend_RateLimiterError проверяет проброс ошибки rate limiter.
func TestSend_RateLimiterError(t *testing.T)  {
	repo := newMockRepository()
	limiter := &mockRateLimiter{err: errors.New("redis down")}
	svc := New(repo, limiter)
	svc.RegisterSender(model.ChannelEmail, &mockSender{})
	
	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	err := svc.Send(context.Background(), n)

	assert.Error(t, err)
}

// TestGetByID_Success проверяет получение уведомления по ID через сервис.
func TestGetByID_Success(t *testing.T)  {
	repo := newMockRepository()
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter)
	svc.RegisterSender(model.ChannelEmail, &mockSender{})
	
	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	_ = svc.Send(context.Background(), n)

	found, err := svc.GetByID(context.Background(), n.ID)

	require.NoError(t, err)
	assert.Equal(t, n.ID, found.ID)
}

// TestGetByID_NotFound проверяет ошибку при запросе несуществующего уведомления.
func TestGetByID_NotFound(t *testing.T)  {
	repo := newMockRepository()
	limiter := &mockRateLimiter{allowed: true}
	svc := New(repo, limiter)
	
	_, err := svc.GetByID(context.Background(), "nonexistent")

	assert.Error(t, err)
}

