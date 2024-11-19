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
	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
)

func main() {
	router := gin.Default()
	router.Use(nrgin.Middleware(NewRelicAgent))
	router.Run(":8000")
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
			traceApp := ""
			if tt.name == "detect and trace gin router in setup function" {
				traceApp = testStatelessTracingFunction(t, tt.code, InstrumentMain)

			} else {
				traceApp = tt.code
			}

			got := testStatelessTracingFunction(t, traceApp, InstrumentGinMiddleware)
			assert.Equal(t, tt.expect, got)
		})
	}
}
