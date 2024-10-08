package main

import (
	"net/http"
)

func main() {
	type test struct {
		err error
	}
	t := test{}
	_, t.err = http.Get("http://example.com")
	if t.err != nil {
		panic(t.err)
	}
}
