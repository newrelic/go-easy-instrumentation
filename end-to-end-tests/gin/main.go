// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func makeGinEndpoint(s string) func(*gin.Context) {
	return func(c *gin.Context) {
		c.Writer.WriteString(s)
	}
}

func v1login(c *gin.Context)  { c.Writer.WriteString("v1 login") }
func v1submit(c *gin.Context) { c.Writer.WriteString("v1 submit") }
func v1read(c *gin.Context)   { c.Writer.WriteString("v1 read") }

func endpoint404(c *gin.Context) {
	c.Writer.WriteHeader(404)
	c.Writer.WriteString("returning 404")
}

func bindingEndpoint(c *gin.Context) {
	type Test struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	testStruct := Test{}
	if err := c.BindJSON(&testStruct); err != nil {
		return
	}

}

func anotherFunc(c *gin.Context) {
	resp, err := http.Get("http://example.com")
	if err != nil {
		slog.Error(err.Error())
	}
	c.Writer.WriteString(resp.Status)
}
func testCallingAnotherFunc(c *gin.Context) {

	anotherFunc(c)
}

func endpointChangeCode(c *gin.Context) {
	// gin.ResponseWriter buffers the response code so that it can be
	// changed before the first write.
	c.Writer.WriteHeader(404)
	c.Writer.WriteHeader(200)
	c.Writer.WriteString("actually ok!")
}

func endpointResponseHeaders(c *gin.Context) {
	// Since gin.ResponseWriter buffers the response code, response headers
	// can be set afterwards.
	c.Writer.WriteHeader(200)
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteString(`{"zip":"zap"}`)
}

func endpointNotFound(c *gin.Context) {
	c.Writer.WriteString("there's no endpoint for that!")
}

func endpointAccessTransaction(c *gin.Context) {
	c.Writer.WriteString("changed the name of the transaction!")
}

func main() {

	router := gin.Default()

	router.GET("/404", endpoint404)
	router.GET("/change", endpointChangeCode)
	router.GET("/headers", endpointResponseHeaders)
	router.GET("/txn", endpointAccessTransaction)
	router.GET("/binding", bindingEndpoint)
	router.GET("/another", testCallingAnotherFunc)

	// Since the handler function name is used as the transaction name,
	// anonymous functions do not get usefully named.  We encourage
	// transforming anonymous functions into named functions.
	router.GET("/anon", func(c *gin.Context) {
		c.Writer.WriteString("anonymous function handler")
	})

	v1 := router.Group("/v1")
	v1.GET("/login", v1login)
	v1.GET("/submit", v1submit)
	v1.GET("/read", v1read)

	router.NoRoute(endpointNotFound)

	router.Run(":8000")
}
