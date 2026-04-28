package redis

import (
	"time"

	"github.com/redis/go-redis/v9"
)

// Config holds Redis connection, auth, and pool settings.
// All fields except Addr are optional — zero values fall back to go-redis defaults.
type Config struct {
	Addr     string // e.g. "localhost:6379"
	Password string // leave empty if auth is not required
	DB       int    // database index (default: 0)

	// Connection pool
	PoolSize     int // default: 10 — max number of socket connections
	MinIdleConns int // default: 5  — minimum idle connections kept in pool
	MaxRetries   int // default: 3  — max retries on transient errors

	// Timeouts
	DialTimeout  time.Duration // default: 5s
	ReadTimeout  time.Duration // default: 3s
	WriteTimeout time.Duration // default: 3s
}

func NewClient(cfg Config) *redis.Client {
	poolSize := cfg.PoolSize
	if poolSize == 0 {
		poolSize = 10
	}
	minIdle := cfg.MinIdleConns
	if minIdle == 0 {
		minIdle = 5
	}
	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}
	dialTimeout := cfg.DialTimeout
	if dialTimeout == 0 {
		dialTimeout = 5 * time.Second
	}
	readTimeout := cfg.ReadTimeout
	if readTimeout == 0 {
		readTimeout = 3 * time.Second
	}
	writeTimeout := cfg.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = 3 * time.Second
	}

	return redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     poolSize,
		MinIdleConns: minIdle,
		MaxRetries:   maxRetries,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})
}
