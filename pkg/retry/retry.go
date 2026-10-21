package retry

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"syscall"
	"time"
)

// Config controls bounded exponential-backoff retries performed in memory.
type Config struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Retryable    func(error) bool
}

func DefaultConfig() Config {
	return Config{MaxAttempts: 3, InitialDelay: 50 * time.Millisecond, MaxDelay: time.Second}
}

// Do retries fn until it succeeds, its context is cancelled, or the bounded
// attempt count is reached. Retry state is intentionally process-local.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = DefaultConfig().MaxAttempts
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = DefaultConfig().InitialDelay
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = DefaultConfig().MaxDelay
	}

	var err error
	delay := cfg.InitialDelay
	if delay > cfg.MaxDelay {
		delay = cfg.MaxDelay
	}
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err = fn(); err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
			(cfg.Retryable != nil && !cfg.Retryable(err)) {
			return err
		}
		if attempt == cfg.MaxAttempts-1 {
			break
		}
		timer := time.NewTimer(time.Duration(rand.Int64N(int64(delay))))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if delay < cfg.MaxDelay {
			if delay > cfg.MaxDelay/2 {
				delay = cfg.MaxDelay
			} else {
				delay *= 2
			}
		}
	}
	return err
}

func IsTransient(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var networkError net.Error
	return (errors.As(err, &networkError) && (networkError.Timeout() || networkError.Temporary())) ||
		errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EPIPE)
}
