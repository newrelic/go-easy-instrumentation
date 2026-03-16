package main

import (
	"log/slog"
	"os"
)

func main() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Custom attribute transformation
			if a.Key == "time" {
				return slog.Attr{}
			}
			return a
		},
	})

	log := slog.New(handler)
	log.Debug("debug message with complex options")
	log.Info("info message", "key1", "value1", "key2", "value2")
}
