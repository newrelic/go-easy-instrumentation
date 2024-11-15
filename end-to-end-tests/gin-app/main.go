// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func anotherFunction() {
	// dummy http request
	_, err := http.Get("https://example.com")

	if err != nil {
		slog.Error(err.Error())
		return
	}
}
func endpointlogic(c *gin.Context) {
	c.Writer.WriteHeader(404)
	c.Writer.WriteString("returning 404")
	// dummy http request
	anotherFunction()
}

func endpoint404(c *gin.Context) {
	c.Writer.WriteHeader(404)
	c.Writer.WriteString("returning 404")
	// dummy http request
	_, err := http.Get("https://example.com")

	if err != nil {
		slog.Error(err.Error())
		return
	}
}
func doSomething() {
	fmt.Println("hi")
	_, err := http.Get("https://example.com")
	if err != nil {
		slog.Error(err.Error())
		return
	}

}
func main() {

	router := gin.Default()

	router.GET("/404", endpoint404)
	router.GET("/logic", endpointlogic)
	router.GET("/anon", func(c *gin.Context) {
		c.Writer.WriteString("anonymous function handler")
		a := func() {
			doSomething()
		}
		a()
		a()
		a()
		// test call
		_, err := http.Get("https://example.com")
		if err != nil {
			slog.Error(err.Error())
			return
		}
	},
		func(c *gin.Context) {
			c.Writer.WriteString("anonymous function handler - second function")
			// test call
			_, err := http.Get("https://example.com")
			if err != nil {
				slog.Error(err.Error())
				return
			}
		})
	router.Run(":8000")
}
