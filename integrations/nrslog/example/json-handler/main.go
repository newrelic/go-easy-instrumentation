package main

import (
	"log/slog"
	"os"
)

func main() {

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	log := slog.New(handler)

	log.Info("JSON formatted message", "key", "value")
	log.Error("JSON error message", "error_code", 500)

}
