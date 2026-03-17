package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDo_ImmediateSuccess(t *testing.T) {
	calls := 0
	err := Do(context.Background(), DefaultConfig(), func(ctx context.Context) error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestDo_SuccessOnThirdAttempt(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{
		MaxAttempts: 5,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     100 * time.Millisecond,
		Multiplier:  2.0,
	}, func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("not yet")
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestDo_MaxRetriesExhausted(t *testing.T) {
	calls := 0
	testErr := errors.New("persistent error")
	err := Do(context.Background(), Config{
		MaxAttempts: 3,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     10 * time.Millisecond,
		Multiplier:  2.0,
	}, func(ctx context.Context) error {
		calls++
		return testErr
	})
	require.Error(t, err)
	assert.Equal(t, testErr, err)
	assert.Equal(t, 3, calls)
}

func TestDo_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()
	err := Do(ctx, Config{
		MaxAttempts: 100,
		InitialWait: 50 * time.Millisecond,
		MaxWait:     1 * time.Second,
		Multiplier:  2.0,
	}, func(ctx context.Context) error {
		calls++
		return errors.New("fail")
	})
	require.Error(t, err)
	assert.True(t, calls >= 1 && calls <= 3)
}

func TestDo_BackoffTiming(t *testing.T) {
	calls := 0
	timestamps := []time.Time{}
	err := Do(context.Background(), Config{
		MaxAttempts: 4,
		InitialWait: 10 * time.Millisecond,
		MaxWait:     1 * time.Second,
		Multiplier:  2.0,
	}, func(ctx context.Context) error {
		calls++
		timestamps = append(timestamps, time.Now())
		if calls < 4 {
			return errors.New("fail")
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 4, calls)

	if len(timestamps) >= 3 {
		gap1 := timestamps[1].Sub(timestamps[0])
		gap2 := timestamps[2].Sub(timestamps[1])
		assert.True(t, gap2 > gap1, "backoff should increase: gap1=%v gap2=%v", gap1, gap2)
	}
}

func TestDo_ZeroAttempts(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{MaxAttempts: 0}, func(ctx context.Context) error {
		calls++
		return errors.New("should not be called")
	})
	require.NoError(t, err)
	assert.Equal(t, 0, calls)
}

func TestDo_MaxWaitCap(t *testing.T) {
	calls := 0
	timestamps := []time.Time{}
	maxWait := 15 * time.Millisecond
	err := Do(context.Background(), Config{
		MaxAttempts: 5,
		InitialWait: 10 * time.Millisecond,
		MaxWait:     maxWait,
		Multiplier:  10.0,
	}, func(ctx context.Context) error {
		calls++
		timestamps = append(timestamps, time.Now())
		if calls < 5 {
			return errors.New("fail")
		}
		return nil
	})
	require.NoError(t, err)

	for i := 2; i < len(timestamps); i++ {
		gap := timestamps[i].Sub(timestamps[i-1])
		assert.True(t, gap < maxWait+10*time.Millisecond, "gap %d should be capped: %v", i, gap)
	}
}

func TestDo_ContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := Do(ctx, DefaultConfig(), func(ctx context.Context) error {
		calls++
		return nil
	})
	require.Error(t, err)
	assert.Equal(t, 0, calls)
}
