package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

func thingy(name string) (*http.Response, error) {
	return http.Get("http://example.com/" + name)
}

func funcThatPaincs() {
	_, err := http.Get("http://example.com/")
	if err != nil {
		panic(err)
	}
}

func main() {
	a := func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)
		if err != nil {
			return err
		}

		funcThatPaincs()

		_, err = thingy("foo")
		if err != nil {
			return err
		}

		_, err = http.DefaultClient.Do(req)
		return err
	}

	err := a(context.Background())
	if err != nil {
		panic(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		fmt.Println("Hello, World async!")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		if err != nil {
			panic(err)
		}
		http.DefaultClient.Do(req)
	}()
	wg.Wait()

	fmt.Println("Done")
}
