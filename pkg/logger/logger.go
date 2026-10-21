package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Level    string
	Format   string
	Sampling bool
}

func (cfg Config) Validate() error {
	_, err := configuration(cfg)
	return err
}

func configuration(cfg Config) (zap.Config, error) {
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	if cfg.Format == "" {
		cfg.Format = "json"
	}
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil || level < zapcore.DebugLevel || level > zapcore.ErrorLevel {
		return zap.Config{}, fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}
	if cfg.Format != "json" && cfg.Format != "console" {
		return zap.Config{}, fmt.Errorf("LOG_FORMAT must be json or console")
	}
	result := zap.NewProductionConfig()
	result.Level = zap.NewAtomicLevelAt(level)
	result.Encoding = cfg.Format
	result.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	if !cfg.Sampling {
		result.Sampling = nil
	}
	return result, nil
}

func NewLogger(cfg Config) (*zap.Logger, error) {
	settings, err := configuration(cfg)
	if err != nil {
		return nil, err
	}
	return settings.Build()
}

func FromContext(ctx context.Context, log *zap.Logger) *zap.Logger {
	fields := make([]zap.Field, 0, 3)
	span := trace.SpanContextFromContext(ctx)
	if span.IsValid() {
		fields = append(fields, zap.String("trace_id", span.TraceID().String()), zap.String("span_id", span.SpanID().String()))
	}
	if correlationID, ok := ctx.Value(correlationKey{}).(string); ok && correlationID != "" {
		fields = append(fields, zap.String("correlationid", correlationID))
	}
	return log.With(fields...)
}

type correlationKey struct{}

func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationKey{}, correlationID)
}

func ReportStartupError(err error) {
	_ = json.NewEncoder(os.Stderr).Encode(map[string]string{
		"level": "error", "msg": "init logger", "error": err.Error(),
	})
}
