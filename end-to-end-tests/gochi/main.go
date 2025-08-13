package main

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func endpoint404(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(404)
	w.Write([]byte("returning 404"))
}

func basicExternal(w http.ResponseWriter, r *http.Request) {
	// Make an http request to an external address
	resp, err := http.Get("https://example.com")
	if err != nil {
		slog.Error(err.Error())
		io.WriteString(w, err.Error())
		return
	}

	defer resp.Body.Close()
	io.Copy(w, resp.Body)
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})
	r.Get("/404", endpoint404)
	r.Get("/external", basicExternal)
	r.Get("/literal", func(w http.ResponseWriter, r *http.Request) {

		_, err := http.Get("https://newrelic.com")
		if err != nil {
			slog.Error(err.Error())
		}
		w.Write([]byte("function literal example"))
	})
	http.ListenAndServe(":3000", r)
}
