package http

import (
	"net/http"
	"time"
)

const defaultTimeout = 30 * time.Second

// NewClient returns an *http.Client with a sensible default timeout.
// Pass 0 to use the default (30 s).
func NewClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &http.Client{Timeout: timeout}
}
