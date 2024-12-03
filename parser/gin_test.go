package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstrumentGinRouter(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "detect and trace gin router in main function",
			code: `package main
	import (
		"github.com/gin-gonic/gin"
	)

	func main() {
		router := gin.Default()
		router.Run(":8000")
	}
`,
			expect: `package main

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	NewRelicAgent, err := newrelic.NewApplication(newrelic.ConfigFromEnvironment())
	if err != nil {
		panic(err)
	}

	router := gin.Default()
	router.Use(nrgin.Middleware(NewRelicAgent))
	router.Run(":8000")

	NewRelicAgent.Shutdown(5 * time.Second)
}
`,
		},
		{
			name: "detect and trace gin router in setup function",
			code: `package main

import (
	"github.com/gin-gonic/gin"
)

func setupRouter(){
	router := gin.Default()
	router.Run(":8000")
}

func main() {
	setupRouter()
}
`,
			expect: `package main

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func setupRouter(nrTxn *newrelic.Transaction) {
	defer nrTxn.StartSegment("setupRouter").End()

	router := gin.Default()
	router.Use(nrgin.Middleware(nrTxn.Application()))
	router.Run(":8000")
}

func main() {
	NewRelicAgent, err := newrelic.NewApplication(newrelic.ConfigFromEnvironment())
	if err != nil {
		panic(err)
	}

	nrTxn := NewRelicAgent.StartTransaction("setupRouter")
	setupRouter(nrTxn)
	nrTxn.End()

	NewRelicAgent.Shutdown(5 * time.Second)
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatelessTracingFunction(t, tt.code, InstrumentMain, InstrumentGinMiddleware)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestGinAnonymousFunction(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "detect and trace gin anonymous function",
			code: `package main
import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()
	router.GET("/anon", func(c *gin.Context) {
		c.Writer.WriteString("anonymous function handler")
		_, err := http.Get("https://example.com")
		if err != nil {
			return
		}
	},
		func(c *gin.Context) {
			c.Writer.WriteString("anonymous function handler - second function")
			_, err := http.Get("https://example.com")
			if err != nil {
				return
			}
		})
	router.Run(":8000")
}
`,
			expect: `package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
)

func main() {
	router := gin.Default()
	router.GET("/anon", func(c *gin.Context) {
		// NR WARN: Since the handler function name is used as the transaction name, anonymous functions do not get usefully named.
		// We encourage transforming anonymous functions into named functions
		nrTxn := nrgin.Transaction(c)
		defer nrTxn.StartSegment("function literal").End()

		c.Writer.WriteString("anonymous function handler")
		_, err := http.Get("https://example.com")
		if err != nil {
			return
		}
	},
		func(c *gin.Context) {
			nrTxn := nrgin.Transaction(c)
			defer nrTxn.StartSegment("function literal").End()

			c.Writer.WriteString("anonymous function handler - second function")
			_, err := http.Get("https://example.com")
			if err != nil {
				return
			}
		})
	router.Run(":8000")
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatelessTracingFunction(t, tt.code, InstrumentGinFunction)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestInstrumentGinFunction(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "detect and trace gin function",
			code: `package main

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleGinFunction(c *gin.Context) {
	c.Writer.WriteString("anonymous function handler")
	// test call
	_, err := http.Get("https://example.com")
	if err != nil {
		slog.Error(err.Error())
		return
	}
}
func main() {
	router := gin.Default()
	router.GET("/anon", HandleGinFunction)
	router.Run(":8000")
}
`,
			expect: `package main

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
)

func HandleGinFunction(c *gin.Context) {
	nrTxn := nrgin.Transaction(c)
	c.Writer.WriteString("anonymous function handler")
	// test call
	_, err := http.Get("https://example.com")
	if err != nil {
		nrTxn.NoticeError(err)
		slog.Error(err.Error())
		return
	}
}
func main() {
	router := gin.Default()
	router.GET("/anon", HandleGinFunction)
	router.Run(":8000")
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatelessTracingFunction(t, tt.code, InstrumentGinFunction)
			assert.Equal(t, tt.expect, got)
		})
	}
}
