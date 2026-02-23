// Package model содержит модели данных сервиса уведомлений.
package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// NotificationStatus представляет статус уведомления в жизненном цикле.
type NotificationStatus string

const (
	// StatusPending — уведомление создано, ожидает обработки.
	StatusPending NotificationStatus = "pending"
	// StatusProcessing — уведомление взято в обработку воркером.
	StatusProcessing NotificationStatus = "processing"
	// StatusSent — уведомление успешно отправлено.
	StatusSent NotificationStatus = "sent"
	// StatusFailed — отправка завершилась ошибкой.
	StatusFailed NotificationStatus = "failed"
)

// IsValid проверяет, является ли статус допустимым.
func (s NotificationStatus) IsValid() bool {
	switch s {
	case StatusPending, StatusProcessing, StatusSent, StatusFailed:
		return true
	}
	return false
}

// Channel представляет канал доставки уведомления.
type Channel string

const (
	// ChannelEmail — отправка по электронной почте.
	ChannelEmail Channel = "email"
	// ChannelPush — push-уведомление.
	ChannelPush Channel = "push"
	// ChannelSMS — отправка SMS.
	ChannelSMS Channel = "sms"
	// ChannelWebhook — отправка через webhook.
	ChannelWebhook Channel = "webhook"
)

// IsValid проверяет, является ли канал допустимым.
func (c Channel) IsValid() bool {
	switch c {
	case ChannelEmail, ChannelPush, ChannelSMS, ChannelWebhook:
		return true
	}
	return false
}

// Notification представляет уведомление для отправки пользователю.
type Notification struct {
	ID         string             `json:"id" db:"id"`
	UserID     string             `json:"user_id" db:"user_id"`
	Channel    Channel            `json:"channel" db:"channel"`
	Payload    string             `json:"payload" db:"payload"`
	Status     NotificationStatus `json:"status" db:"status"`
	RetryCount int                `json:"retry_count" db:"retry_count"`
	LastError  string             `json:"last_error,omitempty" db:"last_error"`
	CreatedAt  time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at" db:"updated_at"`
}

// NewNotification создаёт новое уведомление со статусом pending.
func NewNotification(userID string, channel Channel, payload string) *Notification {
	now := time.Now()
	return &Notification{
		ID:        uuid.New().String(),
		UserID:    userID,
		Channel:   channel,
		Payload:   payload,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

}

// Validate проверяет обязательные поля уведомления.
func (n *Notification) Validate() error {
	if n.UserID == "" {
		return errors.New("user_id is required")
	}
	if !n.Channel.IsValid() {
		return errors.New("invalid channel")
	}
	if n.Payload == "" {
		return errors.New("payload is required")
	}
	return nil
}

// MarkProcessing переводит уведомление в статус processing.
func (n *Notification) MarkProcessing() {
	n.Status = StatusProcessing
	n.UpdatedAt = time.Now()
}

// MarkSent переводит уведомление в статус sent.
func (n *Notification) MarkSent() {
	n.Status = StatusSent
	n.UpdatedAt = time.Now()
}

// MarkFailed переводит уведомление в статус failed, сохраняет ошибку и увеличивает счётчик попыток.
func (n *Notification) MarkFailed(errMsg string) {
	n.Status = StatusFailed
	n.LastError = errMsg
	n.RetryCount++
	n.UpdatedAt = time.Now()
}
