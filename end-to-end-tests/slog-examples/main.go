package main

import (
	"log/slog"
	"os"
)

func main() {

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})
	log := slog.New(handler)

	log.Info("I am a log message")

}
