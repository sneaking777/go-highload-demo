// Package repository реализует слой доступа к данным для уведомлений.
package repository

import (
	"context"
	"fmt"

	"github.com/go-highload-demo/internal/model"
)

// Store определяет интерфейс хранилища уведомлений.
// Реализуется конкретным адаптером (PostgreSQL, in-memory и т.д.).
type Store interface {

	// Save сохраняет уведомление в хранилище.
	Save(ctx context.Context, n *model.Notification) error

	// GetByID возвращает уведомление по идентификатору.
	GetByID(ctx context.Context, id string) (*model.Notification, error)

	// UpdateStatus обновляет статус уведомления и сообщение об ошибке.
	UpdateStatus(ctx context.Context, id string, status model.NotificationStatus, lastError string) error

	// GetPending возвращает до limit уведомлений со статусом pending.
	GetPending(ctx context.Context, limit int) ([] *model.Notification, error)
}

// NotificationRepository предоставляет методы для работы с уведомлениями
// через абстрактное хранилище.
type NotificationRepository struct {
	store Store 
}

// New создаёт новый NotificationRepository с указанным хранилищем.
func New(store Store) *NotificationRepository  {
	return &NotificationRepository{store: store}
}

// Save сохраняет уведомление в хранилище.
func (r *NotificationRepository) Save(ctx context.Context, n *model.Notification) error {
	if err := r.store.Save(ctx, n); err != nil {
		return fmt.Errorf("repository: save failed: %w", err)	
	}
	return nil
}

// GetByID возвращает уведомление по идентификатору.
func (r *NotificationRepository) GetByID(ctx context.Context, id string) (*model.Notification, error) {
	n, err := r.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("repository: get by id failed: %w", err)	
	}
	return n, nil	
}

// UpdateStatus обновляет статус уведомления и сообщение об ошибке.
func (r *NotificationRepository) UpdateStatus(ctx context.Context, id string, status model.NotificationStatus, lastError string) error  {
	if err := r.store.UpdateStatus(ctx, id, status, lastError); err != nil {
		return fmt.Errorf("repository: update status failed: %w", err)	
	}
	return nil	
}

// GetPending возвращает до limit уведомлений со статусом pending.
func (r *NotificationRepository) GetPending(ctx context.Context, limit int) ([]*model.Notification, error)  {
	notifications, err := r.store.GetPending(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("repository: get pending failed: %w", err)
	}
	return notifications, nil	
}