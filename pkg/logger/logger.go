package logger

import "go.uber.org/zap"

// NewLogger returns a production-ready zap logger.
func NewLogger() (*zap.Logger, error) {
	return zap.NewProduction()
}
