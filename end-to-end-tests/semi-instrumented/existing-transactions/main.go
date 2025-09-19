// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func anotherFunc(nrTxn *newrelic.Transaction) {
	fmt.Println("Hello from anotherFunc!")
	nrTxn.Ignore()
}

func hello(nrTxn *newrelic.Transaction) {
	defer nrTxn.StartSegment("hello").End()
	fmt.Println("Hello, World!")
	anotherFunc(nrTxn)
	nrTxn.AddAttribute("color", "red")
	fmt.Println("test")
	nrTxn.End()
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

	nrTxn := app.StartTransaction("hello")
	hello(nrTxn)

	noNRInstrumentation()

	noNRInstrumentation2()
	// Shut down the application to flush data to New Relic.
	app.Shutdown(10 * time.Second)
}
