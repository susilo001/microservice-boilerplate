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
