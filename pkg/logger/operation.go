package logger

import (
	"context"
	"slices"
	"time"

	"github.com/boilerplate/microservice-boilerplate/pkg/apperror"
	"go.uber.org/zap"
)

type OperationConfig struct {
	Component string
	Name      string
}

type Operation struct {
	log       *zap.Logger
	started   time.Time
	degraded  []string
	succeeded bool
}

func StartOperation(ctx context.Context, log *zap.Logger, config OperationConfig, input any) *Operation {
	scoped := FromContext(ctx, log).With(
		zap.String("component", config.Component), zap.String("operation", config.Name),
	)
	operation := &Operation{log: scoped, started: time.Now()}
	DebugPayload(scoped, "operation request", "request", input)
	return operation
}

func (operation *Operation) Debug(message string) {
	operation.log.Debug(message)
}

func (operation *Operation) Degrade(dependency string) {
	if slices.Contains(operation.degraded, dependency) {
		return
	}
	operation.degraded = append(operation.degraded, dependency)
}

func (operation *Operation) Succeed() {
	operation.succeeded = true
}

func (operation *Operation) Finish(output any, err error) {
	fields := []zap.Field{zap.Duration("duration", time.Since(operation.started))}
	if err == nil && !operation.succeeded {
		operation.log.Error("operation interrupted", append(fields, zap.String("outcome", "interrupted"))...)
		DebugPayload(operation.log, "operation response", "response", map[string]any{"error": "interrupted"})
		return
	}
	if err != nil {
		details := apperror.Classify(err)
		outcome := "failed"
		if details.Expected {
			outcome = "rejected"
		}
		fields = append(fields, zap.String("outcome", outcome), zap.String("reason", details.Reason))
		if details.ValidationField != "" {
			fields = append(fields, zap.String("validation_field", details.ValidationField))
		}
		if details.ValidationRule != "" {
			fields = append(fields, zap.String("validation_rule", details.ValidationRule))
		}
		if details.Expected {
			operation.log.Warn("operation rejected", fields...)
		} else {
			operation.log.Error("operation failed", append(fields, zap.Error(err))...)
		}
		DebugPayload(operation.log, "operation response", "response", map[string]any{"error": details.Reason})
		return
	}
	outcome := "success"
	if len(operation.degraded) > 0 {
		outcome = "degraded"
		fields = append(fields, zap.Strings("degraded_dependencies", operation.degraded))
	}
	fields = append(fields, zap.String("outcome", outcome))
	if outcome == "degraded" {
		operation.log.Warn("operation completed", fields...)
	} else {
		operation.log.Info("operation completed", fields...)
	}
	DebugPayload(operation.log, "operation response", "response", output)
}
