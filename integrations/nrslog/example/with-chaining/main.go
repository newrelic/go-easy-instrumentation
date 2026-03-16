package main

import (
	"log/slog"
	"os"
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	logger := slog.New(handler)

	// Test With() chaining preserves instrumentation
	requestLogger := logger.With("request_id", "12345")
	requestLogger.Info("processing request")

	userLogger := requestLogger.With("user_id", "user_67890")
	userLogger.Info("user action", "action", "login")
	userLogger.Warn("user warning", "attempts", 3)
}
