package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-highload-demo/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errNotFound — ошибка, возвращаемая при отсутствии записи.
var errNotFound = errors.New("notification not found")

// mockDB имитирует хранилище уведомлений в памяти для тестов.
type mockDB struct {
	notifications map[string]*model.Notification
	err		error
}

// newMockDB создаёт новый mockDB с пустым хранилищем.
func newMockDB() *mockDB  {
	return &mockDB{notifications: make(map[string]*model.Notification)}
}

func (m *mockDB) Save(ctx context.Context, n *model.Notification) error {
	if m.err != nil {
		return m.err	
	}
	m.notifications[n.ID] = n
	return nil	
}

func (m *mockDB) GetByID(ctx context.Context, id string) (*model.Notification, error)  {
	if m.err != nil {
		return nil, m.err	
	}
	n, ok := m.notifications[id]
	if !ok {
		return nil, errNotFound
	
	}
	return n, nil	
}

func (m *mockDB) UpdateStatus(ctx context.Context, id string, status model.NotificationStatus, lastError string) error  {
	if m.err != nil {
		return m.err	
	}
	n, ok := m.notifications[id]
	if !ok {
		return errNotFound	
	}
	n.Status = status
	n.LastError = lastError
	n.UpdatedAt = time.Now()
	return nil	
}

func (m *mockDB) GetPending(ctx context.Context, limit int) ([]*model.Notification, error)  {
	if m.err != nil {
		return nil, m.err	
	}
	var result []*model.Notification
	for _, n := range m.notifications {
		if n.Status == model.StatusPending {
			result = append(result, n)
			if len(result) >= limit {
				break	
			}	
		}
	}
	return result, nil
}

// TestSave_Success проверяет успешное сохранение уведомления.
func TestSave_Success(t *testing.T)  {
	mock := newMockDB()
	repo := New(mock)
	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	
	err := repo.Save(context.Background(), n)

	require.NoError(t, err)
}

// TestGetByID_Found проверяет получение существующего уведомления по ID.
func TestGetByID_Found(t *testing.T)  {
	mock := newMockDB()
	repo := New(mock)
	n := model.NewNotification("user:1", model.ChannelPush, "hello")
	_ = repo.Save(context.Background(), n)
	
	found, err := repo.GetByID(context.Background(), n.ID)

	require.NoError(t, err)
	assert.Equal(t, n.ID, found.ID)
	assert.Equal(t, n.UserID, found.UserID)
	assert.Equal(t, model.ChannelPush, found.Channel)
}

// TestGetByID_NotFound проверяет ошибку при запросе несуществующего уведомления.
func TestGetByID_NotFound(t *testing.T)  {
	mock := newMockDB()	
	repo := New(mock)

	_, err := repo.GetByID(context.Background(), "nonexistent")

	assert.Error(t, err)
	
}

// TestUpdateStatus_Success проверяет успешное обновление статуса уведомления.
func TestUpdateStatus_Success(t *testing.T)  {
	mock := newMockDB()
	repo := New(mock)
	n := model.NewNotification("user:1", model.ChannelSMS, "hello")
	_ = repo.Save(context.Background(), n)
	
	err := repo.UpdateStatus(context.Background(), n.ID, model.StatusSent, "")

	require.NoError(t, err)
	updated, _ := repo.GetByID(context.Background(), n.ID)
	assert.Equal(t, model.StatusSent, updated.Status)
}

// TestUpdateStatus_NotFound проверяет ошибку при обновлении несуществующего уведомления.
func TestUpdateStatus_NotFound(t *testing.T)  {
	mock := newMockDB()
	repo := New(mock)
	
	err := repo.UpdateStatus(context.Background(), "nonexistent", model.StatusSent, "")

	assert.Error(t, err)
}

// TestGetPending_ReturnsPendingOnly проверяет, что возвращаются только pending-уведомления.
func TestGetPending_ReturnsPendingOnly(t *testing.T)  {
	mock := newMockDB()
	repo := New(mock)

	n1 := model.NewNotification("user:1", model.ChannelEmail, "hello")
	n2 := model.NewNotification("user:2", model.ChannelPush, "world")
	n3 := model.NewNotification("user:3", model.ChannelSMS, "test")
	_ = repo.Save(context.Background(), n1)
	_ = repo.Save(context.Background(), n2)
	_ = repo.Save(context.Background(), n3)

	_ = repo.UpdateStatus(context.Background(), n2.ID, model.StatusSent, "")

	pending, err := repo.GetPending(context.Background(), 10)

	require.NoError(t, err)
	assert.Len(t, pending, 2)
	for _, p := range pending {
		assert.Equal(t, model.StatusPending, p.Status)	
	}
}

// TestGetPending_RespectsLimit проверяет, что limit ограничивает количество результатов.
func TestGetPending_RespectsLimit(t *testing.T)  {
	mock := newMockDB()
	repo := New(mock)
	
	for i := 0; i < 5; i++ {
		n := model.NewNotification("user:1", model.ChannelEmail, "msg")
		_ = repo.Save(context.Background(), n)
	}

	pending, err := repo.GetPending(context.Background(), 3)

	require.NoError(t, err)
	assert.LessOrEqual(t, len(pending), 3)
}

// TestSave_DBError проверяет проброс ошибки хранилища при сохранении.
func TestSave_DBError(t *testing.T) {
	mock := newMockDB()
	mock.err = errors.New("connection refused")
	repo := New(mock)
	
	err := repo.Save(context.Background(), model.NewNotification("user:1", model.ChannelEmail, "hello"))

	assert.Error(t, err)
}

// TestUpdateStatus_WithError проверяет сохранение сообщения об ошибке при обновлении статуса.
func TestUpdateStatus_WithError(t *testing.T)  {
	mock := newMockDB()
	repo := New(mock)
	n := model.NewNotification("user:1", model.ChannelWebhook, "data")
	_ = repo.Save(context.Background(), n)

	err := repo.UpdateStatus(context.Background(), n.ID, model.StatusFailed, "timeout")

	require.NoError(t, err)
	updated, _ := repo.GetByID(context.Background(), n.ID)
	assert.Equal(t, model.StatusFailed, updated.Status)
	assert.Equal(t, "timeout", updated.LastError)	
}
