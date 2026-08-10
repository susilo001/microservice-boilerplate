-- internal/analytics events table.
--
-- Not run via GORM AutoMigrate — partitioning and BRIN indexing aren't
-- well-expressed by it (see AGENTS.md "Schema notes for analytics").
-- Apply manually or via your migration tool of choice against ANALYTICS_DB_DSN.
--
-- Idempotency: Kafka delivers at-least-once. The (event_id, occurred_at)
-- unique constraint + ON CONFLICT DO NOTHING in event_repository.go is what
-- makes redelivery safe.

CREATE TABLE IF NOT EXISTS events (
    event_id    TEXT        NOT NULL,
    user_id     TEXT        NOT NULL,
    event_type  TEXT        NOT NULL,
    payload     JSONB,
    occurred_at TIMESTAMPTZ NOT NULL,
    UNIQUE (event_id, occurred_at)
) PARTITION BY RANGE (occurred_at);

-- BRIN stays tiny for time-ordered inserts at volume, unlike the default
-- B-tree GORM would generate.
CREATE INDEX IF NOT EXISTS idx_events_occurred_at_brin
    ON events USING BRIN (occurred_at);

-- Seed partition. Automate creation of future partitions — see
-- AGENTS.md Good First Task #3 (pg_partman or cmd/scheduler job).
CREATE TABLE IF NOT EXISTS events_default PARTITION OF events DEFAULT;
