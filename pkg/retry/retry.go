package retry

import (
	"context"
	"time"
)

// Config controls bounded exponential-backoff retries performed in memory.
type Config struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
}

func DefaultConfig() Config {
	return Config{MaxAttempts: 3, InitialDelay: 50 * time.Millisecond, MaxDelay: time.Second}
}

// Do retries fn until it succeeds, its context is cancelled, or the bounded
// attempt count is reached. Retry state is intentionally process-local.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	if cfg.MaxAttempts <= 0 {
		cfg = DefaultConfig()
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = DefaultConfig().InitialDelay
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = DefaultConfig().MaxDelay
	}

	var err error
	delay := cfg.InitialDelay
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err = fn(); err == nil {
			return nil
		}
		if attempt == cfg.MaxAttempts-1 {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if delay < cfg.MaxDelay {
			delay *= 2
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}
		}
	}
	return err
}
