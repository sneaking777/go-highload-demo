package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-highload-demo/internal/app"
	"github.com/go-highload-demo/internal/config"
	"github.com/go-highload-demo/internal/repository"
	"github.com/go-highload-demo/internal/storage"
)

// noopLimiter — rate limiter, который всегда разрешает (для тестов).
type noopLimiter struct{}

func (n *noopLimiter) Allow(_ context.Context, _ string) (bool, error)  {
	return true, nil
}

func startTestApp(t *testing.T) (*app.App, string)  {
	t.Helper()
	cfg := &config.Config{
		Server: config.ServerConfig{Addr: ":0"},
	}
	store := storage.NewMemoryStore()
	repo := repository.New(store)
	
	a := app.New(cfg, repo, &noopLimiter{})
	addr, err := a.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.Shutdown(ctx)
	})

	return a, addr
}


func TestCreateAndGetNotification(t *testing.T) {
	_, addr := startTestApp(t)

    // POST /notifications — создаём уведомление	
	body := `{"user_id":"user1","channel":"email","payload":"hello"}`
	resp, err := http.Post("http://"+addr+"/notifications", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	id := result["id"]
	assert.NotEmpty(t, id)

	// GET /notifications/{id} — получаем созданное
	resp2, err := http.Get("http://" + addr + "/notifications/" + id)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var notification map[string]interface{}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&notification))
	assert.Equal(t, id, notification["id"])
	assert.Equal(t, "sent", notification["status"])
}

func TestNotificationNotFound(t *testing.T)  {
	_, addr := startTestApp(t)
	
	resp, err := http.Get("http://" + addr + "/notifications/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestInvalidJSON(t *testing.T)  {
	_, addr := startTestApp(t)

	resp, err := http.Post("http://"+addr+"/notifications", "application/json", bytes.NewBufferString("{invalid"))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)	
}

func TestGracefulShutdown(t *testing.T)  {
	a, addr := startTestApp(t)
	
	// Сервер обслуживает запросы
    resp, err := http.Get("http://" + addr + "/notifications/test")
	require.NoError(t, err)
	resp.Body.Close()
	
	// Корректное завершение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = a.Shutdown(ctx)
	assert.NoError(t, err)

	// После shutdown соединения отклоняются
	time.Sleep(100 * time.Millisecond)
	_, err = http.Get("http://" + addr + "/notifications/test")
	assert.Error(t, err)
}