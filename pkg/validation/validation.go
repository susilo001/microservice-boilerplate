package validation

import (
	"errors"
	"fmt"
	"strings"
)

type Error struct {
	Field  string
	Rule   string
	Cause  error
	Source error
}

func (err *Error) Error() string {
	detail := fmt.Sprintf("%s failed %s validation", err.Field, err.Rule)
	if err.Cause != nil {
		return err.Cause.Error() + ": " + detail
	}
	return detail
}

func (err *Error) Unwrap() error {
	if err.Source != nil {
		return errors.Join(err.Cause, err.Source)
	}
	return err.Cause
}

func Required(field, value string, cause error) error {
	if strings.TrimSpace(value) == "" {
		return &Error{Field: field, Rule: "required", Cause: cause}
	}
	return nil
}
