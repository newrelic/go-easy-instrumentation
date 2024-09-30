package main

import (
	"log/slog"
	"net/http"
	"os"
)

// design pattern that forces awareness of call depth to pass instrumentation
func initServer() {
	http.HandleFunc("/external", external)

}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	slog.SetDefault(logger)

	slog.Info("starting server at localhost:8000")
	initServer()

	http.ListenAndServe(":8000", nil)
}
