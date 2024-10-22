package main

import (
	"http-app/pkg"
	"log/slog"
	"net/http"
	"os"
)

// design pattern that forces awareness of call depth to pass instrumentation
func initServer() {
	http.HandleFunc("/", index)
	http.HandleFunc("/error", noticeError)
	http.HandleFunc("/external", external)
	http.HandleFunc("/roundtrip", roundtripper)
	http.HandleFunc("/basicExternal", basicExternal)
	http.HandleFunc("/async", async)
	http.HandleFunc("/async2", async2)
	http.HandleFunc("/packaged", pkg.PackagedHandler)

	// this should no longer get ignored
	DoAThing(true)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	slog.SetDefault(logger)

	slog.Info("starting server at localhost:8000")
	initServer()

	http.ListenAndServe(":8000", nil)
}
