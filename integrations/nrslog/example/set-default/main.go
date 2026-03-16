package main

import (
	"log/slog"
	"os"
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	slog.SetDefault(slog.New(handler))

	// Package-level calls now work with the default logger
	slog.Info("using default logger")
	slog.Error("error with default logger", "code", 500)
	slog.Warn("warning message")
}
