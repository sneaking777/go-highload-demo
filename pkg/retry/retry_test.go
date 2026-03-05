package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDo_SuccessFirstAttempt проверяет, что при успешном вызове retry не повторяет попытку.
func TestDo_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{MaxAttempts: 3}, func(ctx context.Context) error {
		calls++
		return nil
	})
	
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

// TestDo_SuccessAfterRetries проверяет, что retry повторяет вызов до первого успеха.
func TestDo_SuccessAfterRetries(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{MaxAttempts: 5, BaseDelay: time.Millisecond}, func (ctx context.Context) error  {
		calls++
		if calls < 3 {
			return errors.New("temporary error")	
		}
		return nil	
	})
	
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

// TestDo_AllAttemptsFailed проверяет, что при исчерпании попыток возвращается последняя ошибка.
func TestDo_AllAttemptsFailed(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{MaxAttempts: 3, BaseDelay: time.Millisecond}, func (ctx context.Context) error  {
		calls++
		return errors.New("persistent error")		
	})
	
	require.Error(t, err)
	assert.Equal(t, "persistent error", err.Error())
	assert.Equal(t, 3, calls)
}

// TestDo_ContextCancelled проверяет, что retry прекращается при отмене контекста.
func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	
	err := Do(ctx, Config{MaxAttempts: 10, BaseDelay: 50 * time.Millisecond}, func (ctx context.Context) error  {
		calls++
		if calls == 2 {
			cancel()	
		}
		return errors.New("error")	
	})

	require.Error(t, err)
	assert.LessOrEqual(t, calls, 3)
}

// TestDo_ExponentialBackoff проверяет, что задержка растёт экспоненциально.
func TestDo_ExponentialBackoff(t *testing.T) {
	calls := 0
	timestamps := make([]time.Time, 0)

	_ = Do(context.Background(), Config{
		MaxAttempts: 4,
		BaseDelay: 10 * time.Millisecond,
		MaxDelay: time.Second,
		Multiplier: 2.0,
	}, func (ctx context.Context) error  {
		timestamps = append(timestamps, time.Now())
		calls++
		return errors.New("error")		
	})

	require.Equal(t, 4, calls)

	// Проверяем что каждая следующая задержка >= предыдущей
	for i := 2; i < len(timestamps); i++ {
		prev := timestamps[i-1].Sub(timestamps[i-2])
		curr := timestamps[i].Sub(timestamps[i-1])
		assert.GreaterOrEqual(t, curr.Milliseconds(), prev.Milliseconds(), "delay #%d should be >= delay #%d", i, i-1)
	}
}

// TestDo_MaxDelayCap проверяет, что задержка не превышает MaxDelay.
func TestDo_MaxDelayCap(t *testing.T) {
	timestamps := make([]time.Time, 0)

	_ = Do(context.Background(), Config{
		MaxAttempts: 5,
		BaseDelay: 10 * time.Millisecond,
		MaxDelay: 15 * time.Millisecond,
		Multiplier: 2.0,
	}, func (ctx context.Context) error  {
		timestamps = append(timestamps, time.Now())
		return errors.New("error")	
	})

	for i := 1; i < len(timestamps); i++ {
		delay := timestamps[i].Sub(timestamps[i-1])
		assert.LessOrEqual(t, delay.Milliseconds(), int64(30), "delay should not greatly exceed MaxDelay")
	}
}