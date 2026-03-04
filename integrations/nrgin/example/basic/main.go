package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
	"github.com/newrelic/go-agent/v3/newrelic"
)

var db = make(map[string]string)

func setupRouter(nrTxn *newrelic.Transaction) *gin.Engine {
	defer nrTxn.StartSegment("setupRouter").End()

	// Disable Console Color
	// gin.DisableConsoleColor()
	r := gin.Default()

	r.Use(nrgin.Middleware(nrTxn.Application()))

	// Ping test
	// NR WARN: function literal segments will be named "function literal" by default
	// declare a function instead to improve segment name generation
	r.GET("/ping", func(c *gin.Context) {
		nrTxn := nrgin.Transaction(c)
		defer nrTxn.StartSegment("function literal").End()

		c.String(http.StatusOK, "pong")
		// the "http.Get()" net/http method can not be instrumented and its outbound traffic can not be traced
		// please see these examples of code patterns for external http calls that can be instrumented:
		// https://docs.newrelic.com/docs/apm/agents/go-agent/configuration/distributed-tracing-go-agent/#make-http-requests
		//
		// make a dummy request and err check
		_, err := http.Get("http://localhost:8080/ping")
		if err != nil {
			nrTxn.NoticeError(err)
			c.String(http.StatusInternalServerError, "error")
			return
		}
	})
	// two test
	// NR WARN: function literal segments will be named "function literal" by default
	// declare a function instead to improve segment name generation
	//
	// NR WARN: function literal segments will be named "function literal" by default
	// declare a function instead to improve segment name generation
	r.GET("/", func(c *gin.Context) {
		nrTxn := nrgin.Transaction(c)
		defer nrTxn.StartSegment("function literal").End()

		c.String(http.StatusOK, "pong")
	}, func(c *gin.Context) {
		nrTxn := nrgin.Transaction(c)
		defer nrTxn.StartSegment("function literal").End()

		c.String(http.StatusOK, "second function")
	})

	// Get user value
	// NR WARN: function literal segments will be named "function literal" by default
	// declare a function instead to improve segment name generation
	r.GET("/user/:name", func(c *gin.Context) {
		nrTxn := nrgin.Transaction(c)
		defer nrTxn.StartSegment("function literal").End()

		user := c.Params.ByName("name")
		value, ok := db[user]
		if ok {
			c.JSON(http.StatusOK, gin.H{"user": user, "value": value})
		} else {
			c.JSON(http.StatusOK, gin.H{"user": user, "status": "no value"})
		}
	})

	// Authorized group (uses gin.BasicAuth() middleware)
	// Same than:
	// authorized := r.Group("/")
	// authorized.Use(gin.BasicAuth(gin.Credentials{
	//	  "foo":  "bar",
	//	  "manu": "123",
	//}))
	authorized := r.Group("/", gin.BasicAuth(gin.Accounts{
		"foo":  "bar", // user:foo password:bar
		"manu": "123", // user:manu password:123
	}))

	/* example curl for /admin with basicauth header
	   Zm9vOmJhcg== is base64("foo:bar")

		curl -X POST \
	  	http://localhost:8080/admin \
	  	-H 'authorization: Basic Zm9vOmJhcg==' \
	  	-H 'content-type: application/json' \
	  	-d '{"value":"bar"}'
	*/
	// NR WARN: function literal segments will be named "function literal" by default
	// declare a function instead to improve segment name generation
	authorized.POST("admin", func(c *gin.Context) {
		nrTxn := nrgin.Transaction(c)
		defer nrTxn.StartSegment("function literal").End()

		user := c.MustGet(gin.AuthUserKey).(string)

		// Parse JSON
		var json struct {
			Value string `json:"value" binding:"required"`
		}

		if c.Bind(&json) == nil {
			db[user] = json.Value
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		}
	})

	return r
}

func main() {
	NewRelicAgent, agentInitError := newrelic.NewApplication(newrelic.ConfigFromEnvironment())
	if agentInitError != nil {
		panic(agentInitError)
	}

	nrTxn := NewRelicAgent.StartTransaction("setupRouter")
	r := setupRouter(nrTxn)
	nrTxn.End()
	// Listen and Server in 0.0.0.0:8080
	r.Run(":8080")

	NewRelicAgent.Shutdown(5 * time.Second)
}
