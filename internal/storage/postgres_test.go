package storage_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-highload-demo/internal/model"
	"github.com/go-highload-demo/internal/storage"
)

const migrationSQL = `CREATE TABLE IF NOT EXISTS notifications (
	id         VARCHAR(36) PRIMARY KEY,
	user_id    VARCHAR(255) NOT NULL,
	channel    VARCHAR(50)  NOT NULL,
	payload    TEXT         NOT NULL,
	status     VARCHAR(50)  NOT NULL DEFAULT 'pending',
	retry_count INT         NOT NULL DEFAULT 0,
	last_error TEXT         NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ  NOT NULL,
	updated_at TIMESTAMPTZ  NOT NULL
)`

func setupPostgres(t *testing.T) *storage.PostgresStore {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set, skipping postgres integration test")
	}

	ctx := context.Background()
	store, err := storage.NewPostgresStore(ctx, dsn)
	require.NoError(t, err)

	require.NoError(t, store.Migrate(ctx, migrationSQL))
	// Очищаем таблицу перед тестом
	require.NoError(t, store.Migrate(ctx, "DELETE FROM notifications"))

	t.Cleanup(func() { store.Close() })
	return store
}

func TestPostgres_SaveAndGetByID(t *testing.T) {
	store := setupPostgres(t)
	ctx := context.Background()

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	require.NoError(t, store.Save(ctx, n))

	found, err := store.GetByID(ctx, n.ID)
	require.NoError(t, err)
	assert.Equal(t, n.ID, found.ID)
	assert.Equal(t, n.UserID, found.UserID)
	assert.Equal(t, n.Channel, found.Channel)
	assert.Equal(t, n.Payload, found.Payload)
	assert.Equal(t, model.StatusPending, found.Status)
}

func TestPostgres_GetByID_NotFound(t *testing.T) {
	store := setupPostgres(t)
	ctx := context.Background()

	_, err := store.GetByID(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestPostgres_UpdateStatus(t *testing.T) {
	store := setupPostgres(t)
	ctx := context.Background()

	n := model.NewNotification("user:1", model.ChannelPush, "test")
	require.NoError(t, store.Save(ctx, n))

	require.NoError(t, store.UpdateStatus(ctx, n.ID, model.StatusSent, ""))
	found, err := store.GetByID(ctx, n.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusSent, found.Status)
}

func TestPostgres_UpdateStatus_Failed(t *testing.T) {
	store := setupPostgres(t)
	ctx := context.Background()

	n := model.NewNotification("user:1", model.ChannelSMS, "test")
	require.NoError(t, store.Save(ctx, n))

	require.NoError(t, store.UpdateStatus(ctx, n.ID, model.StatusFailed, "timeout"))
	found, err := store.GetByID(ctx, n.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusFailed, found.Status)
	assert.Equal(t, "timeout", found.LastError)
}

func TestPostgres_UpdateStatus_NotFound(t *testing.T) {
	store := setupPostgres(t)
	ctx := context.Background()

	err := store.UpdateStatus(ctx, "nonexistent", model.StatusSent, "")
	assert.Error(t, err)
}

func TestPostgres_GetPending(t *testing.T) {
	store := setupPostgres(t)
	ctx := context.Background()

	// Создаём 3 уведомления: 2 pending, 1 sent
	n1 := model.NewNotification("user:1", model.ChannelEmail, "p1")
	n1.CreatedAt = time.Now().Add(-2 * time.Second)
	n2 := model.NewNotification("user:2", model.ChannelPush, "p2")
	n2.CreatedAt = time.Now().Add(-1 * time.Second)
	n3 := model.NewNotification("user:3", model.ChannelSMS, "p3")

	require.NoError(t, store.Save(ctx, n1))
	require.NoError(t, store.Save(ctx, n2))
	require.NoError(t, store.Save(ctx, n3))
	require.NoError(t, store.UpdateStatus(ctx, n3.ID, model.StatusSent, ""))

	pending, err := store.GetPending(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, pending, 2)

	// С лимитом 1
	pending, err = store.GetPending(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, pending, 1)
}
