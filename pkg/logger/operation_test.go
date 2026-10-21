package logger

import (
	"context"
	"errors"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/pkg/apperror"
	"github.com/boilerplate/microservice-boilerplate/pkg/validation"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestOperationAcrossComponentsAndOutcomes(t *testing.T) {
	rejected := &validation.Error{
		Field: "address", Rule: "required",
		Cause: &apperror.Error{Code: "invalid_address", Message: "business rejection"},
	}
	for _, component := range []string{"orders.usecase", "notifications.gateway"} {
		for _, test := range []struct {
			name       string
			err        error
			complete   bool
			dependency string
			level      zapcore.Level
			outcome    string
			reason     string
		}{
			{"success", nil, true, "", zap.InfoLevel, "success", ""},
			{"degraded", nil, true, "queue", zap.WarnLevel, "degraded", ""},
			{"rejection", rejected, false, "", zap.WarnLevel, "rejected", "invalid_address"},
			{"cancellation", context.Canceled, false, "", zap.WarnLevel, "rejected", "request_canceled"},
			{"deadline", context.DeadlineExceeded, false, "", zap.WarnLevel, "rejected", "deadline_exceeded"},
			{"failure", errors.New("unavailable"), false, "", zap.ErrorLevel, "failed", "operation_failed"},
			{"interrupted", nil, false, "", zap.ErrorLevel, "interrupted", ""},
		} {
			t.Run(component+"/"+test.name, func(t *testing.T) {
				core, entries := observer.New(zap.DebugLevel)
				ctx := WithCorrelationID(context.Background(), "operation-request")
				operation := StartOperation(ctx, zap.New(core), OperationConfig{
					Component: component, Name: "Submit",
				}, map[string]any{"id": "item-1"})
				if test.dependency != "" {
					operation.Degrade(test.dependency)
					operation.Degrade(test.dependency)
				}
				operation.Debug("validation passed")
				if test.complete {
					operation.Succeed()
				}
				operation.Finish(map[string]any{"id": "item-1"}, test.err)
				summaries := entries.FilterLevelExact(test.level).All()
				if len(summaries) != 1 {
					t.Fatalf("expected one outcome summary: %+v", entries.All())
				}
				fields := summaries[0].ContextMap()
				if fields["outcome"] != test.outcome || fields["component"] != component ||
					fields["operation"] != "Submit" || fields["correlationid"] != "operation-request" {
					t.Fatalf("unexpected operation fields: %v", fields)
				}
				if test.reason != "" && fields["reason"] != test.reason {
					t.Fatalf("reason=%v, want %s", fields["reason"], test.reason)
				}
				if test.err == rejected && (fields["validation_field"] != "address" || fields["validation_rule"] != "required") {
					t.Fatalf("missing caller-provided validation details: %v", fields)
				}
				if test.dependency != "" {
					dependencies := fields["degraded_dependencies"].([]any)
					if len(dependencies) != 1 || dependencies[0] != test.dependency {
						t.Fatalf("unexpected degraded dependencies: %v", dependencies)
					}
				}
				if entries.FilterMessage("operation request").Len() != 1 || entries.FilterMessage("operation response").Len() != 1 {
					t.Fatal("missing Debug payloads")
				}
			})
		}
	}
}

func TestOperationWithDefaultClassification(t *testing.T) {
	core, entries := observer.New(zap.WarnLevel)
	operation := StartOperation(context.Background(), zap.New(core), OperationConfig{
		Component: "worker", Name: "Consume",
	}, nil)
	operation.Finish(nil, context.DeadlineExceeded)
	fields := entries.All()[0].ContextMap()
	if fields["reason"] != "deadline_exceeded" || fields["outcome"] != "rejected" {
		t.Fatalf("unexpected default classification: %v", fields)
	}
}
