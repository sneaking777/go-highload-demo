// Package handler реализует HTTP-обработчики сервиса уведомлений.
package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-highload-demo/internal/model"
)

// Service определяет интерфейс бизнес-логики, используемой обработчиками.
type Service interface {
	// Send выполняет отправку уведомления.
	Send(ctx context.Context, n *model.Notification) error
	// GetByID возвращает уведомление по идентификатору.
	GetByID(ctx context.Context, id string) (*model.Notification, error)
}

// Handler содержит HTTP-обработчики для работы с уведомлениями.
type Handler struct {
	svc Service
}

// New создаёт новый Handler с указанным сервисом.
func New(svc Service) *Handler {
	return &Handler{svc: svc}
}

// createRequest представляет тело запроса на создание уведомления.
type createRequest struct {
	UserID  string        `json:"user_id"`
	Channel model.Channel `json:"channel"`
	Payload string        `json:"payload"`
}

// CreateNotification обрабатывает POST /notifications.
// Декодирует JSON, валидирует поля, отправляет уведомление и возвращает 201 с ID.
func (h *Handler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	n := model.NewNotification(req.UserID, req.Channel, req.Payload)
	if err := n.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	if err := h.svc.Send(r.Context(), n); err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": n.ID})
}

// GetNotification обрабатывает GET /notifications/{id}.
// Возвращает уведомление в JSON или 404 если не найдено.
func (h *Handler) GetNotification(w http.ResponseWriter, r *http.Request, id string) {
	n, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(n)
}
