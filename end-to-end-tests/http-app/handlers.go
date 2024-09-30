package main

import (
	"io"
	"net/http"
)

func external(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequest("GET", "https://example.com", nil)

	// Make an http request to an external address
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	defer resp.Body.Close()
	io.Copy(w, resp.Body)
}
