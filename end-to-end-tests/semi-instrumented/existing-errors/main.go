// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func hello(txn *newrelic.Transaction) {
	defer txn.StartSegment("hello").End()
	// make a mock http call with err return
	_, err := http.Get("https://example.com")
	if err != nil {
		txn.NoticeError(err)
	}
	txn.End()
}
func errorNotTraced() {
	_, err := http.Get("https://example.com")
	if err != nil {
		fmt.Println("Error occurred:", err)
	}
}

func noNRInstrumentation() {
	fmt.Println("hi")
}

func noNRInstrumentation2() {
	fmt.Println("hi")
}
func main() {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("Short Lived App"),
		newrelic.ConfigLicense(os.Getenv("NEW_RELIC_LICENSE_KEY")),
		newrelic.ConfigDebugLogger(os.Stdout),
	)
	if nil != err {
		fmt.Println(err)
		os.Exit(1)
	}

	// Wait for the application to connect.
	if err := app.WaitForConnection(5 * time.Second); nil != err {
		fmt.Println(err)
	}

	// Do the tasks at hand.  Perhaps record them using transactions and/or
	// custom events.
	tasks := []string{"white", "black", "red", "blue", "green", "yellow"}
	for _, task := range tasks {
		txn := app.StartTransaction("task")
		time.Sleep(10 * time.Millisecond)
		fmt.Println("Task:", task)
		txn.End()
		app.RecordCustomEvent("task", map[string]interface{}{
			"color": task,
		})
	}

	txn := app.StartTransaction("hello")
	hello(txn)
	errorNotTraced()
	noNRInstrumentation()

	noNRInstrumentation2()
	// Shut down the application to flush data to New Relic.
	app.Shutdown(10 * time.Second)
}
