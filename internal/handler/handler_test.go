package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-highload-demo/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockService имитирует сервис уведомлений для тестов.
type mockService struct {
	sendCalled bool
	sendErr    error
	getResult  *model.Notification
	getErr     error
}

func (m *mockService) Send(ctx context.Context, n *model.Notification) error {
	m.sendCalled = true
	return m.sendErr
}

func (m *mockService) GetByID(ctx context.Context, id string) (*model.Notification, error) {
	return m.getResult, m.getErr
}

// TestCreateNotification_Success проверяет успешное создание уведомления через POST.
func TestCreateNotification_Success(t *testing.T) {
	svc := &mockService{}
	h := New(svc)

	body := `{"user_id":"user:1","channel":"email","payload":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/notifications", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateNotification(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, svc.sendCalled)
}

// TestCreateNotification_InvalidJSON проверяет ошибку при невалидном JSON в теле запроса.
func TestCreateNotification_InvalidJSON(t *testing.T) {
	svc := &mockService{}
	h := New(svc)

	req := httptest.NewRequest(http.MethodPost, "/notifications", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateNotification(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateNotification_ValidationError проверяет ошибку при отсутствии обязательных полей.
func TestCreateNotification_ValidationError(t *testing.T) {
	svc := &mockService{}
	h := New(svc)

	body := `{"user_id":"","channel":"email","payload":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/notifications", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateNotification(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, svc.sendCalled)
}

// TestCreateNotification_ServiceError проверяет ответ 500 при ошибке сервиса.
func TestCreateNotification_ServiceError(t *testing.T) {
	svc := &mockService{sendErr: errors.New("service down")}
	h := New(svc)

	body := `{"user_id":"user:1","channel":"email","payload":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/notifications", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateNotification(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetNotification_Success проверяет успешное получение уведомления по ID.
func TestGetNotification_Success(t *testing.T) {
	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	svc := &mockService{getResult: n}
	h := New(svc)

	req := httptest.NewRequest(http.MethodGet, "/notifications/"+n.ID, nil)
	w := httptest.NewRecorder()

	h.GetNotification(w, req, n.ID)

	require.Equal(t, http.StatusOK, w.Code)

	var result model.Notification
	err := json.NewDecoder(w.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, n.ID, result.ID)
}

// TestGetNotification_NotFound проверяет ответ 404 при запросе несуществующего уведомления.
func TestGetNotification_NotFound(t *testing.T) {
	svc := &mockService{getErr: errors.New("not found")}
	h := New(svc)

	req := httptest.NewRequest(http.MethodGet, "/notifications/nonexistent", nil)
	w := httptest.NewRecorder()

	h.GetNotification(w, req, "nonexistent")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestCreateNotification_ResponseContainsID проверяет, что ответ содержит ID созданного уведомления.
func TestCreateNotification_ResponseContainsID(t *testing.T) {
	svc := &mockService{}
	h := New(svc)

	body := `{"user_id":"user:1","channel":"push","payload":"data"}`
	req := httptest.NewRequest(http.MethodPost, "/notifications", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateNotification(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp["id"])
}
