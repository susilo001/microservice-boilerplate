package databases

import (
	"errors"
	"fmt"
	"strings"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	otelgorm "github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"
)

// Config holds database connection, pool, and optional read-replica settings.
// All fields except Driver and DSN are optional — zero values use sane defaults.
type Config struct {
	// Driver selects the database engine.
	// Supported values: "postgres" | "mysql" | "sqlite" | "sqlserver"
	Driver string

	// Primary (source) connection string.
	// Format depends on the driver:
	//   postgres   — "postgres://user:pass@host:5432/db?sslmode=disable"
	//   mysql      — "user:pass@tcp(host:3306)/db?charset=utf8mb4&parseTime=True&loc=Local"
	//   sqlite     — file path, e.g. "app.db" or ":memory:"
	//   sqlserver  — "sqlserver://user:pass@host:1433?database=db"
	DSN string

	// ReplicaDSNs lists one or more read-replica DSNs (same driver as primary).
	// DBResolver routes SELECT queries to replicas (random policy) and all
	// writes to the primary. Not applicable to SQLite.
	ReplicaDSNs []string

	// Connection pool — applied to primary and each replica.
	// Ignored for SQLite (which is file-based and not pool-managed).
	MaxOpenConns    int           // default: 25
	MaxIdleConns    int           // default: 10
	ConnMaxLifetime time.Duration // default: 5m
	ConnMaxIdleTime time.Duration // default: 5m
}

// NewDB opens a GORM connection for the configured driver, applies pool
// settings, registers otelgorm instrumentation, and (when ReplicaDSNs is set)
// wires DBResolver for automatic read/write splitting.
func NewDB(cfg Config) (*gorm.DB, error) {
	dial, err := dialector(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(dial, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("connect db (%s): %w", cfg.Driver, err)
	}

	maxOpen, maxIdle, maxLifetime, maxIdleTime := resolvedPool(cfg)

	if !isSQLite(cfg.Driver) {
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("get sql.DB: %w", err)
		}
		sqlDB.SetMaxOpenConns(maxOpen)
		sqlDB.SetMaxIdleConns(maxIdle)
		sqlDB.SetConnMaxLifetime(maxLifetime)
		sqlDB.SetConnMaxIdleTime(maxIdleTime)
	}

	if err := db.Use(otelgorm.NewPlugin()); err != nil {
		return nil, fmt.Errorf("init otelgorm: %w", err)
	}

	if len(cfg.ReplicaDSNs) > 0 && !isSQLite(cfg.Driver) {
		replicas := make([]gorm.Dialector, len(cfg.ReplicaDSNs))
		for i, dsn := range cfg.ReplicaDSNs {
			d, err := dialector(cfg.Driver, dsn)
			if err != nil {
				return nil, fmt.Errorf("replica[%d]: %w", i, err)
			}
			replicas[i] = d
		}

		resolver := dbresolver.Register(dbresolver.Config{
			Replicas: replicas,
			Policy:   dbresolver.RandomPolicy{},
		}).
			SetMaxOpenConns(maxOpen).
			SetMaxIdleConns(maxIdle).
			SetConnMaxLifetime(maxLifetime).
			SetConnMaxIdleTime(maxIdleTime)

		if err := db.Use(resolver); err != nil {
			return nil, fmt.Errorf("init dbresolver: %w", err)
		}
	}

	return db, nil
}

// dialector maps a driver name to the corresponding GORM Dialector.
func dialector(driver, dsn string) (gorm.Dialector, error) {
	switch strings.ToLower(driver) {
	case "postgres", "postgresql":
		return postgres.Open(dsn), nil
	case "mysql":
		return mysql.Open(dsn), nil
	case "sqlite", "sqlite3":
		return gsqlite.Open(dsn), nil
	case "sqlserver", "mssql":
		return sqlserver.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported db driver %q (supported: postgres, mysql, sqlite, sqlserver)", driver)
	}
}

func isSQLite(driver string) bool {
	d := strings.ToLower(driver)
	return d == "sqlite" || d == "sqlite3"
}

func resolvedPool(cfg Config) (maxOpen, maxIdle int, maxLifetime, maxIdleTime time.Duration) {
	maxOpen = cfg.MaxOpenConns
	if maxOpen == 0 {
		maxOpen = 25
	}
	maxIdle = cfg.MaxIdleConns
	if maxIdle == 0 {
		maxIdle = 10
	}
	maxLifetime = cfg.ConnMaxLifetime
	if maxLifetime == 0 {
		maxLifetime = 5 * time.Minute
	}
	maxIdleTime = cfg.ConnMaxIdleTime
	if maxIdleTime == 0 {
		maxIdleTime = 5 * time.Minute
	}
	return
}

func Close(db *gorm.DB) error {
	var closeErrors []error
	for _, plugin := range db.Config.Plugins {
		if resolver, ok := plugin.(*dbresolver.DBResolver); ok {
			_ = resolver.Call(func(pool gorm.ConnPool) error {
				if closer, ok := pool.(interface{ Close() error }); ok {
					closeErrors = append(closeErrors, closer.Close())
				}
				return nil
			})
		}
	}
	pool, err := db.DB()
	if err != nil {
		closeErrors = append(closeErrors, err)
	} else {
		closeErrors = append(closeErrors, pool.Close())
	}
	return errors.Join(closeErrors...)
}
