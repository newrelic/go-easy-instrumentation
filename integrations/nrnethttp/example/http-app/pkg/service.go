package pkg

import (
	"fmt"
	"log/slog"
	"net/http"
)

func Service() error {
	req, err := buildGetRequest("https://example.com")
	if err != nil {
		return err
	}

	// Make an http request to an external address
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	return nil
}

func buildGetRequest(path string) (*http.Request, error) {
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		errMsg := fmt.Sprintf("failed to build request: %v", err)
		slog.Error(errMsg)
		return nil, fmt.Errorf(errMsg)
	}
	return req, nil
}

func PackagedHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("I am a packaged handler"))
}
