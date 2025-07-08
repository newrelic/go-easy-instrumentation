package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstrumentChiRouter(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "detect and trace chi router in main function",
			code: `package main
import (
	"net/http"

	chi "github.com/go-chi/chi/v5"
)

func main() {
	router := chi.NewRouter()
	http.ListenAndServe(":3000", router)
}
`,
			expect: `package main

import (
	"net/http"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/newrelic/go-agent/v3/integrations/nrgochi"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	NewRelicAgent, agentInitError := newrelic.NewApplication(newrelic.ConfigFromEnvironment())
	if agentInitError != nil {
		panic(agentInitError)
	}

	router := chi.NewRouter()
	router.Use(nrgochi.Middleware(NewRelicAgent))
	http.ListenAndServe(":3000", router)

	NewRelicAgent.Shutdown(5 * time.Second)
}
`,
		},
		{
			name: "detect and trace chi router in setup function",
			code: `package main
import (
	"net/http"

	chi "github.com/go-chi/chi/v5"
)

func setupRouter() {
	router := chi.NewRouter()
	http.ListenAndServe(":3000", router)
}

func main() {
	setupRouter()
}
`,

			expect: `package main

import (
	"net/http"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/newrelic/go-agent/v3/integrations/nrgochi"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func setupRouter(nrTxn *newrelic.Transaction) {
	defer nrTxn.StartSegment("setupRouter").End()

	router := chi.NewRouter()
	router.Use(nrgochi.Middleware(nrTxn.Application()))
	http.ListenAndServe(":3000", router)
}

func main() {
	NewRelicAgent, agentInitError := newrelic.NewApplication(newrelic.ConfigFromEnvironment())
	if agentInitError != nil {
		panic(agentInitError)
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
			got := testStatelessTracingFunction(t, tt.code, InstrumentMain, InstrumentChiMiddleware)
			assert.Equal(t, tt.expect, got)
		})
	}
}
