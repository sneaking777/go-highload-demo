// Package sender реализует отправщики уведомлений по различным каналам доставки.
package sender

import (
	"context"
	"testing"
	"time"

	"github.com/go-highload-demo/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmailSender_Send проверяет успешную отправку email-уведомления.
func TestEmailSender_Send(t *testing.T) {
	s := NewEmailSender()
	err := s.Send(context.Background(), model.NewNotification("user1", model.ChannelEmail, "hello"))
	assert.NoError(t, err)
}

// TestPushSender_Send проверяет успешную отправку push-уведомления.
func TestPushSender_Send(t *testing.T) {
	s := NewPushSender()
	err := s.Send(context.Background(), model.NewNotification("user1", model.ChannelPush, "hello"))
	assert.NoError(t, err)
}

// TestSMSSender_Send проверяет успешную отправку SMS-уведомления.
func TestSMSSender_Send(t *testing.T) {
	s := NewSMSSender()
	err := s.Send(context.Background(), model.NewNotification("user1", model.ChannelSMS, "hello"))
	assert.NoError(t, err)
}

// TestWebhookSender_Send проверяет успешную отправку webhook-уведомления.
func TestWebhookSender_Send(t *testing.T) {
	s := NewWebhookSender()
	err := s.Send(context.Background(), model.NewNotification("user1", model.ChannelWebhook, "hello"))
	assert.NoError(t, err)
}

// TestSender_RespectsContextCancellation проверяет, что Send возвращает ошибку при отменённом контексте.
func TestSender_RespectsContextCancellation(t *testing.T) {
	s := NewEmailSender()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Send(ctx, model.NewNotification("user1", model.ChannelEmail, "hello"))
	assert.Error(t, err)
}

// TestRegistry_GetSender проверяет получение отправщика из реестра по каналу.
func TestRegistry_GetSender(t *testing.T) {
	r := NewRegistry()
	r.Register(model.ChannelEmail, NewEmailSender())
	r.Register(model.ChannelPush, NewPushSender())

	s, ok := r.Get(model.ChannelEmail)
	require.True(t, ok)
	assert.NotNil(t, s)

	_, ok = r.Get(model.ChannelSMS)
	assert.False(t, ok)
}

// TestSender_SimulatesLatency проверяет, что отправка имитирует сетевую задержку.
func TestSender_SimulatesLatency(t *testing.T) {
	s := NewEmailSender()
	start := time.Now()
	_ = s.Send(context.Background(), model.NewNotification("user1", model.ChannelEmail, "hello"))
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, 10*time.Millisecond, "отправка должна имитировать задержку")
}
