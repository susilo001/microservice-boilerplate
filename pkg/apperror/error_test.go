package apperror

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/boilerplate/microservice-boilerplate/pkg/validation"
)

func TestClassify(t *testing.T) {
	rejected := &Error{Code: "invalid_order", Message: "invalid order"}
	for _, test := range []struct {
		name string
		err  error
		want Details
	}{
		{"wrapped rejection", fmt.Errorf("submit: %w", rejected), Details{Reason: "invalid_order", Expected: true}},
		{"validation", fmt.Errorf("submit: %w", validation.Required("id", "", rejected)), Details{
			Reason: "invalid_order", Expected: true, ValidationField: "id", ValidationRule: "required",
		}},
		{"canceled", fmt.Errorf("submit: %w", context.Canceled), Details{Reason: "request_canceled", Expected: true}},
		{"deadline", context.DeadlineExceeded, Details{Reason: "deadline_exceeded", Expected: true}},
		{"unknown", errors.New("connection unavailable"), Details{Reason: "operation_failed"}},
		{"empty code", &Error{Message: "unclassified"}, Details{Reason: "operation_failed"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Classify(test.err); got != test.want {
				t.Fatalf("Classify() = %+v, want %+v", got, test.want)
			}
		})
	}
	if !errors.Is(fmt.Errorf("submit: %w", rejected), rejected) || rejected.Error() != "invalid order" {
		t.Fatal("application error lost sentinel identity or message")
	}
}
