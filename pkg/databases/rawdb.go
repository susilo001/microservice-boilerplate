package databases

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RawConfig configures the analytics domain's direct pgxpool connection.
// Postgres-only — COPY isn't portable across the multi-driver GORM abstraction.
type RawConfig struct {
	DSN      string
	MaxConns int32 // default: 10
}

// NewRawPool opens a pgxpool for internal/analytics/repository's bulk COPY
// writes. Never used outside that package — see AGENTS.md "GORM exception".
func NewRawPool(ctx context.Context, cfg RawConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse analytics dsn: %w", err)
	}

	maxConns := cfg.MaxConns
	if maxConns == 0 {
		maxConns = 10
	}
	poolCfg.MaxConns = maxConns
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("connect analytics db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping analytics db: %w", err)
	}
	return pool, nil
}
