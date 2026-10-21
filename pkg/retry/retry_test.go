package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoRetriesUntilSuccess(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Config{
		MaxAttempts:  3,
		InitialDelay: time.Millisecond,
		MaxDelay:     time.Millisecond,
	}, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestDoReturnsLastErrorAfterExhaustion(t *testing.T) {
	want := errors.New("unavailable")
	err := Do(context.Background(), Config{
		MaxAttempts:  2,
		InitialDelay: time.Millisecond,
		MaxDelay:     time.Millisecond,
	}, func() error {
		return want
	})

	if !errors.Is(err, want) {
		t.Errorf("Do() error = %v, want %v", err, want)
	}
}

func TestDoStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Do(ctx, Config{
		MaxAttempts:  2,
		InitialDelay: time.Second,
		MaxDelay:     time.Second,
	}, func() error {
		return errors.New("temporary failure")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Do() error = %v, want %v", err, context.Canceled)
	}
}

func TestDoDoesNotRetryPermanentErrors(t *testing.T) {
	attempts := 0
	want := errors.New("permanent")
	policy := DefaultConfig()
	policy.Retryable = IsTransient
	err := Do(context.Background(), policy, func() error {
		attempts++
		return want
	})
	if !errors.Is(err, want) || attempts != 1 {
		t.Fatalf("error=%v, attempts=%d; want permanent error after one attempt", err, attempts)
	}
}

func TestDoStopsDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	attempts := 0
	err := Do(ctx, DefaultConfig(), func() error {
		attempts++
		cancel()
		return errors.New("unavailable")
	})
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("error=%v, attempts=%d; want cancellation after one attempt", err, attempts)
	}
}

func TestDoCapsInitialDelay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	attempts := 0
	err := Do(ctx, Config{MaxAttempts: 2, InitialDelay: time.Hour, MaxDelay: time.Nanosecond}, func() error {
		attempts++
		if attempts == 1 {
			return errors.New("retry")
		}
		return nil
	})
	if err != nil || attempts != 2 {
		t.Fatalf("error=%v, attempts=%d; initial delay was not capped", err, attempts)
	}
}
