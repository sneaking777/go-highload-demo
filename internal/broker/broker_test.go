package broker_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-highload-demo/internal/broker"
	"github.com/go-highload-demo/internal/model"
)

// --- LocalBroker ---

// TestLocalBroker_PublishAndProcess проверяет, что LocalBroker доставляет уведомление обработчику.
func TestLocalBroker_PublishAndProcess(t *testing.T) {
	var mu sync.Mutex
	var processed []*model.Notification

	b := broker.NewLocalBroker(2, 10)
	ctx := context.Background()

	require.NoError(t, b.Run(ctx, func(ctx context.Context, n *model.Notification) {
		mu.Lock()
		defer mu.Unlock()
		processed = append(processed, n)
	}))
	defer b.Shutdown()

	n := model.NewNotification("user:1", model.ChannelEmail, "hello")
	require.NoError(t, b.Publish(ctx, n))

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(processed) == 1
	}, time.Second, 10*time.Millisecond)
}

// TestLocalBroker_MultipleMessages проверяет обработку нескольких сообщений.
func TestLocalBroker_MultipleMessages(t *testing.T) {
	var mu sync.Mutex
	count := 0

	b := broker.NewLocalBroker(4, 100)
	ctx := context.Background()

	require.NoError(t, b.Run(ctx, func(ctx context.Context, n *model.Notification) {
		mu.Lock()
		defer mu.Unlock()
		count++
	}))
	defer b.Shutdown()

	for i := 0; i < 50; i++ {
		n := model.NewNotification("user:1", model.ChannelEmail, "msg")
		require.NoError(t, b.Publish(ctx, n))
	}

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return count == 50
	}, 2*time.Second, 10*time.Millisecond)
}

// TestLocalBroker_Shutdown проверяет graceful shutdown.
func TestLocalBroker_Shutdown(t *testing.T) {
	b := broker.NewLocalBroker(1, 10)
	ctx := context.Background()

	require.NoError(t, b.Run(ctx, func(ctx context.Context, n *model.Notification) {}))

	err := b.Shutdown()
	assert.NoError(t, err)
}

// --- RabbitMQBroker (integration) ---

// TestRabbitMQBroker_PublishAndConsume проверяет публикацию и потребление через RabbitMQ.
func TestRabbitMQBroker_PublishAndConsume(t *testing.T) {
	url := os.Getenv("TEST_RABBITMQ_URL")
	if url == "" {
		t.Skip("TEST_RABBITMQ_URL not set, skipping rabbitmq integration test")
	}

	b, err := broker.NewRabbitMQBroker(url)
	require.NoError(t, err)
	defer b.Shutdown()

	var mu sync.Mutex
	var received *model.Notification

	ctx := context.Background()
	require.NoError(t, b.Run(ctx, func(ctx context.Context, n *model.Notification) {
		mu.Lock()
		defer mu.Unlock()
		received = n
	}))

	n := model.NewNotification("user:rmq", model.ChannelPush, "rabbitmq-test")
	require.NoError(t, b.Publish(ctx, n))

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return received != nil && received.ID == n.ID
	}, 5*time.Second, 50*time.Millisecond)
}
