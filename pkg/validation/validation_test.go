package validation

import (
	"errors"
	"fmt"
	"testing"
)

func TestRequired(t *testing.T) {
	cause := errors.New("invalid input")
	for _, value := range []string{"", " ", "\t\n", "item-1", " item-1 "} {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			err := Required("id", value, cause)
			if value == "item-1" || value == " item-1 " {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var detail *Error
			if !errors.Is(err, cause) || !errors.As(err, &detail) || detail.Field != "id" || detail.Rule != "required" {
				t.Fatalf("unexpected validation error: %v", err)
			}
			if err.Error() != "invalid input: id failed required validation" {
				t.Fatalf("unexpected message: %s", err)
			}
		})
	}
}

func TestRequiredWithoutCause(t *testing.T) {
	err := Required("name", "", nil)
	if err == nil || err.Error() != "name failed required validation" || errors.Unwrap(err) != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
