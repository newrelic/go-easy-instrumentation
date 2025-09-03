package main

import (
	"context"
	"errors"
	"http-app/pkg"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

// the most basic http handler function
func index(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "hello world")
}

func anotherFuncWithAContext(ctx context.Context) {
	val := ctx.Value("key")
	if val != nil {
		slog.Info("we found the key!", slog.Any("key", val))
	}
}

func aFunctionWithContextArguments(ctx context.Context) {
	// do something
	err := ctx.Err()
	if err != nil {
		return
	}

	DoAThing(false)
	anotherFuncWithAContext(ctx)
}

func DoAThing(willError bool) (string, bool, error) {
	time.Sleep(200 * time.Millisecond)
	if willError {
		return "thing not done", false, errors.New("this is an error")
	}

	return "thing complete", true, nil
}

func noticeError(w http.ResponseWriter, r *http.Request) {
	err := pkg.Service()
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	aFunctionWithContextArguments(r.Context())

	str, _, err := DoAThing(true)
	if err != nil {
		io.WriteString(w, err.Error())
	} else {
		io.WriteString(w, str+" no errors occured")
	}
}

func external(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequest("GET", "https://example.com", nil)
	if err != nil {
		slog.Error(err.Error())
		return
	}

	// Make an http request to an external address
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	defer resp.Body.Close()
	io.Copy(w, resp.Body)
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

func roundtripper(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{}
	client2 := client // verify that this doesn't get the transport replaced by the parser

	request, err := http.NewRequest("GET", "https://example.com", nil)
	if err != nil {
		slog.Error(err.Error())
		return
	}

	resp, err := client2.Do(request)

	// this is an unusual spacing and comment pattern to test the decoration preservation
	if err != nil {
		slog.Error(err.Error())
		io.WriteString(w, err.Error())
		return
	}

	defer resp.Body.Close()
	io.Copy(w, resp.Body)
}

func async(w http.ResponseWriter, r *http.Request) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(100 * time.Millisecond)
	}()
	wg.Wait()
	w.Write([]byte("done!"))
}

func doAsyncThing(wg *sync.WaitGroup) {
	defer wg.Done()
	time.Sleep(100 * time.Millisecond)
	_, err := http.Get("http://example.com")
	if err != nil {
		slog.Error(err.Error())
	}
}

func async2(w http.ResponseWriter, r *http.Request) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go doAsyncThing(wg)

	go aFunctionWithContextArguments(r.Context())
	wg.Wait()
	w.Write([]byte("done!"))
}

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
	http.HandleFunc("init-lit", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello init"))
	})

	// this should no longer get ignored
	DoAThing(true)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	slog.SetDefault(logger)

	slog.Info("starting server at localhost:8000")
	http.HandleFunc("/main-lit", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello main"))
	})
	initServer()

	http.ListenAndServe(":8000", nil)
}
