package kafka

import (
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// WriterConfig holds Kafka producer settings.
// All fields except Brokers are optional — zero values fall back to sane defaults.
type WriterConfig struct {
	Brokers []string

	// Batching — tune for throughput vs. latency
	BatchSize    int           // default: 100  — max messages per batch
	BatchTimeout time.Duration // default: 1s   — max wait before flushing a partial batch

	// Delivery guarantees
	// "all"    → WaitForAll  (strongest, default)
	// "leader" → WaitForLeader
	// "none"   → RequireNone (fire-and-forget)
	RequiredAcks string

	// Async disables the writer's internal flushing loop; callers must call
	// WriteMessages themselves. Set true only when you manage flushing explicitly.
	Async bool

	// Compression algorithm: "none" (default), "gzip", "snappy", "lz4", "zstd"
	Compression string
}

func NewWriter(cfg WriterConfig) *kafka.Writer {
	batchSize := cfg.BatchSize
	if batchSize == 0 {
		batchSize = 100
	}
	batchTimeout := cfg.BatchTimeout
	if batchTimeout == 0 {
		batchTimeout = time.Second
	}

	return &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    batchSize,
		BatchTimeout: batchTimeout,
		RequiredAcks: parseRequiredAcks(cfg.RequiredAcks),
		Async:        cfg.Async,
		Compression:  parseCompression(cfg.Compression),
	}
}

func parseRequiredAcks(s string) kafka.RequiredAcks {
	switch strings.ToLower(s) {
	case "leader":
		return kafka.RequireOne
	case "none":
		return kafka.RequireNone
	default: // "all" or empty
		return kafka.RequireAll
	}
}

func parseCompression(s string) kafka.Compression {
	switch strings.ToLower(s) {
	case "gzip":
		return kafka.Gzip
	case "snappy":
		return kafka.Snappy
	case "lz4":
		return kafka.Lz4
	case "zstd":
		return kafka.Zstd
	default: // "none" or empty
		return 0
	}
}
