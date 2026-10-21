package apperror

import (
	"context"
	"errors"

	"github.com/boilerplate/microservice-boilerplate/pkg/validation"
)

type Error struct {
	Code    string
	Message string
}

func (err *Error) Error() string {
	return err.Message
}

type Details struct {
	Reason          string
	Expected        bool
	ValidationField string
	ValidationRule  string
}

func Classify(err error) Details {
	details := Details{Reason: "operation_failed"}
	var applicationError *Error
	switch {
	case errors.Is(err, context.Canceled):
		details = Details{Reason: "request_canceled", Expected: true}
	case errors.Is(err, context.DeadlineExceeded):
		details = Details{Reason: "deadline_exceeded", Expected: true}
	case errors.As(err, &applicationError) && applicationError.Code != "":
		details = Details{Reason: applicationError.Code, Expected: true}
	}
	var validationError *validation.Error
	if errors.As(err, &validationError) {
		details.ValidationField = validationError.Field
		details.ValidationRule = validationError.Rule
	}
	return details
}
