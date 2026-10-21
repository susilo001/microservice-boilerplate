package logger

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLoggerConfiguration(t *testing.T) {
	for _, test := range []struct {
		name   string
		config Config
		level  zapcore.Level
		format string
	}{
		{"defaults", Config{}, zap.InfoLevel, "json"},
		{"debug console", Config{Level: "debug", Format: "console"}, zap.DebugLevel, "console"},
		{"warn json", Config{Level: "warn", Format: "json"}, zap.WarnLevel, "json"},
		{"error json", Config{Level: "error", Format: "json"}, zap.ErrorLevel, "json"},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings, err := configuration(test.config)
			if err != nil {
				t.Fatal(err)
			}
			if settings.Encoding != test.format || settings.Sampling != nil || settings.Development {
				t.Fatalf("unexpected logging policy: %+v", settings)
			}
			log, err := NewLogger(test.config)
			if err != nil {
				t.Fatal(err)
			}
			for level := zap.DebugLevel; level <= zap.ErrorLevel; level++ {
				if got, want := log.Core().Enabled(level), level >= test.level; got != want {
					t.Errorf("level %v enabled=%v, want %v", level, got, want)
				}
			}
		})
	}
	settings, err := configuration(Config{Sampling: true})
	if err != nil || settings.Sampling == nil {
		t.Fatalf("sampling was not enabled explicitly: %v", err)
	}
}

func TestLoggerRejectsInvalidConfiguration(t *testing.T) {
	for _, cfg := range []Config{{Level: "typo"}, {Level: "fatal"}, {Format: "xml"}} {
		if _, err := NewLogger(cfg); err == nil {
			t.Fatalf("accepted invalid configuration: %+v", cfg)
		}
	}
}

func TestFromContextPreservesCorrelationAndTrace(t *testing.T) {
	core, entries := observer.New(zap.DebugLevel)
	traceID, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID,
	}))
	ctx = WithCorrelationID(ctx, "request-1")
	FromContext(ctx, zap.New(core)).Warn("adapter failure")
	fields := entries.All()[0].ContextMap()
	if fields["trace_id"] != traceID.String() || fields["span_id"] != spanID.String() ||
		fields["correlationid"] != "request-1" {
		t.Fatalf("missing logging context: %v", fields)
	}
}
