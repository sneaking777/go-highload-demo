// Package storage реализует конкретные адаптеры хранилищ для сервиса уведомлений.
package storage

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-highload-demo/internal/model"	
)

// MemoryStore — потокобезопасное хранилище уведомлений в оперативной памяти.
// Реализует интерфейс repository.Store.
type MemoryStore struct {
	mu sync.RWMutex
	data map[string]*model.Notification
}

// NewMemoryStore создаёт новое пустое хранилище в памяти.
func NewMemoryStore() *MemoryStore  {
	return &MemoryStore{
		data: make(map[string]*model.Notification),
	}
}

// Save сохраняет уведомление в памяти.
func (m *MemoryStore) Save(ctx context.Context, n *model.Notification) error  {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[n.ID] = n
	return nil
}

// GetByID возвращает уведомление по идентификатору.
func (m *MemoryStore) GetByID(ctx context.Context, id string) (*model.Notification, error)  {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.data[id]
	if !ok {
		return nil, fmt.Errorf("notification %s not found", id)	
	}
	return n, nil
}

// UpdateStatus обновляет статус уведомления.
func (m *MemoryStore) UpdateStatus(ctx context.Context, id string, status model.NotificationStatus, lastError string) error  {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.data[id]
	if !ok {
		return fmt.Errorf("notification %s not found", id)		
	}
	switch status {
	case model.StatusProcessing:
		n.MarkProcessing()
	case model.StatusSent:
		n.MarkSent()
	case model.StatusFailed:
		n.MarkFailed(lastError)		
	}
	return nil
}

// GetPending возвращает до limit уведомлений со статусом pending.
func (m *MemoryStore) GetPending(ctx context.Context, limit int) ([]*model.Notification, error)  {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*model.Notification
	for _, n := range m.data {
		if n.Status == model.StatusPending {
			result = append(result, n)
			if len(result) >= limit {
				break	
			}	
		}		
	}
	return result, nil
}
