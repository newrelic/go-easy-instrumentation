package nrlogrus_test

import (
	"testing"

	"github.com/newrelic/go-easy-instrumentation/integrations/nrlogrus"
	"github.com/newrelic/go-easy-instrumentation/parser"

	"github.com/stretchr/testify/assert"
)

// TestInstrumentLogrusHandler_LoggerSetFormatter exercises pattern 1:
// a custom logger with an explicit SetFormatter call gets its argument wrapped.
func TestInstrumentLogrusHandler_LoggerSetFormatter(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "json_formatter_wrapped",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.Info("hello")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.JSONFormatter{}))
	logger.Info("hello")
}
`,
		},
		{
			name: "text_formatter_wrapped",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{})
	logger.Warn("warn")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logger.Warn("warn")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrlogrus.InstrumentLogrusHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestInstrumentLogrusHandler_LoggerNoSetFormatter exercises pattern 2:
// a custom logger with no SetFormatter gets a default injected after logrus.New().
func TestInstrumentLogrusHandler_LoggerNoSetFormatter(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "single_logger",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	logger := logrus.New()
	logger.Info("hello")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logger.Info("hello")
}
`,
		},
		{
			name: "multiple_loggers_inject_each",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	a := logrus.New()
	b := logrus.New()
	a.Info("a")
	b.Info("b")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	a := logrus.New()
	a.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	b := logrus.New()
	b.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	a.Info("a")
	b.Info("b")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrlogrus.InstrumentLogrusHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestInstrumentLogrusHandler_PackageSetFormatter exercises pattern 3:
// a package-level logrus.SetFormatter call gets its argument wrapped.
func TestInstrumentLogrusHandler_PackageSetFormatter(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "package_set_formatter_wrapped",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.Info("hello")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logrus.Info("hello")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrlogrus.InstrumentLogrusHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestInstrumentLogrusHandler_StandardLoggerOnly exercises pattern 4:
// only logrus.Info / etc are used (no logger var, no SetFormatter), and a
// logrus.SetFormatter is injected before the first logrus call.
func TestInstrumentLogrusHandler_StandardLoggerOnly(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "info_only_injects_set_formatter",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	logrus.Info("hello")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logrus.Info("hello")
}
`,
		},
		{
			name: "first_call_among_many",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	logrus.Info("first")
	logrus.Warn("second")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logrus.Info("first")
	logrus.Warn("second")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrlogrus.InstrumentLogrusHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestInstrumentLogrusHandler_Idempotent verifies that re-running the
// instrumentation on already-instrumented code is a no-op.
func TestInstrumentLogrusHandler_Idempotent(t *testing.T) {
	code := `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.JSONFormatter{}))
	logger.Info("hello")
}`
	expect := `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.JSONFormatter{}))
	logger.Info("hello")
}
`
	defer parser.PanicRecovery(t)
	got := parser.RunStatelessTracingFunction(t, code, nrlogrus.InstrumentLogrusHandler)
	assert.Equal(t, expect, got)
}

// TestInstrumentLogrusHandler_NoLogrusUsage verifies that code with no logrus
// references is left untouched.
func TestInstrumentLogrusHandler_NoLogrusUsage(t *testing.T) {
	code := `package main
import "fmt"
func main() {
	fmt.Println("hello")
}`
	expect := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	defer parser.PanicRecovery(t)
	got := parser.RunStatelessTracingFunction(t, code, nrlogrus.InstrumentLogrusHandler)
	assert.Equal(t, expect, got)
}
