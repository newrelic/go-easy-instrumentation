package main

import (
	"log/slog"
	"os"
)

func main() {

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})
	handler2 := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})
	log := slog.New(handler)
	log2 := slog.New(handler2)

	log.Info("I am a log message")
	log2.Warn("Another message")

}
