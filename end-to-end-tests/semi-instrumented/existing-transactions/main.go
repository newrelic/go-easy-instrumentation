// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func anotherFunc(txn *newrelic.Transaction) {
	fmt.Println("Hello from anotherFunc!")
	txn.Ignore()
}

func hello(txn *newrelic.Transaction) {
	defer txn.StartSegment("hello").End()
	fmt.Println("Hello, World!")
	anotherFunc(txn)
	txn.AddAttribute("color", "red")
	fmt.Println("test")
	txn.End()
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

	noNRInstrumentation()

	noNRInstrumentation2()
	// Shut down the application to flush data to New Relic.
	app.Shutdown(10 * time.Second)
}
