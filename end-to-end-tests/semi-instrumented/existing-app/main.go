// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func index(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "hello world")
}

func noticeErrorWithAttributes(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Noticing an error")

	err := errors.New("error with attributes")
	if err != nil {
		fmt.Println(err)
	}
}

func startNRApp() (*newrelic.Application, error) {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("Example App"),
		newrelic.ConfigFromEnvironment(),
	)
	return app, err
}

func main() {
	myapp, err := startNRApp()
	if err != nil {
		panic(err)
	}
	myapp.WaitForConnection(5 * time.Second)

	http.HandleFunc("/", index)
	http.HandleFunc("/notice_error_with_attributes", noticeErrorWithAttributes)

	http.ListenAndServe(":8000", nil)
}
