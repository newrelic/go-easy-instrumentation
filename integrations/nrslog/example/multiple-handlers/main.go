package main

import (
	"log/slog"
	"os"
)

func main() {
	debugHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	errorHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	})

	debugLog := slog.New(debugHandler)
	errorLog := slog.New(errorHandler)

	debugLog.Debug("debug info to stdout")
	debugLog.Info("info message to stdout")
	errorLog.Error("critical error to stderr", "code", 500)
}
