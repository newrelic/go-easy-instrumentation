package nrslog_test

import (
	"testing"

	"github.com/newrelic/go-easy-instrumentation/integrations/nrslog"
	"github.com/newrelic/go-easy-instrumentation/parser"

	"github.com/stretchr/testify/assert"
)

// TestInstrumentSlogHandler_TextHandler tests basic TextHandler wrapping
func TestInstrumentSlogHandler_TextHandler(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "wrap_text_handler_in_main",
			code: `package main
import (
	"log/slog"
	"os"
)
func main() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	log := slog.New(handler)
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	NRhandler := nrslog.WrapHandler(NewRelicAgent, handler)
	log := slog.New(NRhandler)
}
`,
		},
		{
			name: "wrap_text_handler_with_options",
			code: `package main
import (
	"log/slog"
	"os"
)
func main() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	log := slog.New(handler)
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	NRhandler := nrslog.WrapHandler(NewRelicAgent, handler)
	log := slog.New(NRhandler)
}
`,
		},
		{
			name: "multiple_text_handlers",
			code: `package main
import (
	"log/slog"
	"os"
)
func main() {
	handler1 := slog.NewTextHandler(os.Stdout, nil)
	handler2 := slog.NewTextHandler(os.Stderr, nil)
	log1 := slog.New(handler1)
	log2 := slog.New(handler2)
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

func main() {
	handler1 := slog.NewTextHandler(os.Stdout, nil)
	NRhandler1 := nrslog.WrapHandler(NewRelicAgent, handler1)
	handler2 := slog.NewTextHandler(os.Stderr, nil)
	NRhandler2 := nrslog.WrapHandler(NewRelicAgent, handler2)
	log1 := slog.New(NRhandler1)
	log2 := slog.New(NRhandler2)
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrslog.InstrumentSlogHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestInstrumentSlogHandler_JSONHandler tests JSONHandler wrapping
func TestInstrumentSlogHandler_JSONHandler(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "wrap_json_handler_in_main",
			code: `package main
import (
	"log/slog"
	"os"
)
func main() {
	handler := slog.NewJSONHandler(os.Stdout, nil)
	log := slog.New(handler)
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

func main() {
	handler := slog.NewJSONHandler(os.Stdout, nil)
	NRhandler := nrslog.WrapHandler(NewRelicAgent, handler)
	log := slog.New(NRhandler)
}
`,
		},
		{
			name: "wrap_json_handler_with_options",
			code: `package main
import (
	"log/slog"
	"os"
)
func main() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})
	log := slog.New(handler)
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

func main() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})
	NRhandler := nrslog.WrapHandler(NewRelicAgent, handler)
	log := slog.New(NRhandler)
}
`,
		},
		{
			name: "mixed_json_and_text_handlers",
			code: `package main
import (
	"log/slog"
	"os"
)
func main() {
	jsonHandler := slog.NewJSONHandler(os.Stdout, nil)
	textHandler := slog.NewTextHandler(os.Stderr, nil)
	jsonLog := slog.New(jsonHandler)
	textLog := slog.New(textHandler)
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

func main() {
	jsonHandler := slog.NewJSONHandler(os.Stdout, nil)
	NRjsonHandler := nrslog.WrapHandler(NewRelicAgent, jsonHandler)
	textHandler := slog.NewTextHandler(os.Stderr, nil)
	NRtextHandler := nrslog.WrapHandler(NewRelicAgent, textHandler)
	jsonLog := slog.New(NRjsonHandler)
	textLog := slog.New(NRtextHandler)
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrslog.InstrumentSlogHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestInstrumentSlogHandler_InitFunction tests handler initialization in init()
func TestInstrumentSlogHandler_InitFunction(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "wrap_handler_in_init",
			code: `package main
import (
	"log/slog"
	"os"
)
var logger *slog.Logger
func init() {
	handler := slog.NewJSONHandler(os.Stdout, nil)
	logger = slog.New(handler)
}
func main() {
	logger.Info("message")
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

var logger *slog.Logger

func init() {
	handler := slog.NewJSONHandler(os.Stdout, nil)
	NRhandler := nrslog.WrapHandler(NewRelicAgent, handler)
	logger = slog.New(NRhandler)
}
func main() {
	logger.Info("message")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrslog.InstrumentSlogHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestInstrumentSlogHandler_HelperFunction tests handler initialization in helper functions
func TestInstrumentSlogHandler_HelperFunction(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "wrap_handler_in_helper_function",
			code: `package main
import (
	"log/slog"
	"os"
)
func setupLogger() *slog.Logger {
	handler := slog.NewTextHandler(os.Stdout, nil)
	return slog.New(handler)
}
func main() {
	log := setupLogger()
	log.Info("message")
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

func setupLogger() *slog.Logger {
	handler := slog.NewTextHandler(os.Stdout, nil)
	NRhandler := nrslog.WrapHandler(NewRelicAgent, handler)
	return slog.New(NRhandler)
}
func main() {
	log := setupLogger()
	log.Info("message")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrslog.InstrumentSlogHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestInstrumentSlogHandler_SetDefault tests slog.SetDefault() pattern
func TestInstrumentSlogHandler_SetDefault(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "set_default_with_text_handler",
			code: `package main
import (
	"log/slog"
	"os"
)
func main() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	slog.SetDefault(slog.New(handler))
	slog.Info("using default")
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	NRhandler := nrslog.WrapHandler(NewRelicAgent, handler)
	slog.SetDefault(slog.New(NRhandler))
	slog.Info("using default")
}
`,
		},
		{
			name: "set_default_with_json_handler",
			code: `package main
import (
	"log/slog"
	"os"
)
func main() {
	handler := slog.NewJSONHandler(os.Stdout, nil)
	slog.SetDefault(slog.New(handler))
	slog.Error("error message")
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

func main() {
	handler := slog.NewJSONHandler(os.Stdout, nil)
	NRhandler := nrslog.WrapHandler(NewRelicAgent, handler)
	slog.SetDefault(slog.New(NRhandler))
	slog.Error("error message")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrslog.InstrumentSlogHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestInstrumentSlogHandler_ComplexOptions tests complex HandlerOptions
func TestInstrumentSlogHandler_ComplexOptions(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "handler_with_replace_attr",
			code: `package main
import (
	"log/slog"
	"os"
)
func main() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return a
		},
	})
	log := slog.New(handler)
	log.Info("message")
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

func main() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return a
		},
	})
	NRhandler := nrslog.WrapHandler(NewRelicAgent, handler)
	log := slog.New(NRhandler)
	log.Info("message")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrslog.InstrumentSlogHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestInstrumentSlogHandler_WithChaining tests With() method chaining
func TestInstrumentSlogHandler_WithChaining(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "with_chaining",
			code: `package main
import (
	"log/slog"
	"os"
)
func main() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	logger := slog.New(handler)
	requestLogger := logger.With("request_id", "123")
	requestLogger.Info("processing")
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	NRhandler := nrslog.WrapHandler(NewRelicAgent, handler)
	logger := slog.New(NRhandler)
	requestLogger := logger.With("request_id", "123")
	requestLogger.Info("processing")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrslog.InstrumentSlogHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestInstrumentSlogHandler_NoInstrumentation tests cases where no instrumentation should occur
func TestInstrumentSlogHandler_NoInstrumentation(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "no_slog_usage",
			code: `package main
import "fmt"
func main() {
	fmt.Println("hello")
}`,
			expect: `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`,
		},
		{
			name: "only_slog_new_without_handler",
			code: `package main
import (
	"log/slog"
	"os"
)
func main() {
	log := slog.New(nil)
	log.Info("message")
}`,
			expect: `package main

import "log/slog"

func main() {
	log := slog.New(nil)
	log.Info("message")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrslog.InstrumentSlogHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestInstrumentSlogHandler_EdgeCases tests various edge cases
func TestInstrumentSlogHandler_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "handler_variable_reuse",
			code: `package main
import (
	"log/slog"
	"os"
)
func main() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	logger1 := slog.New(handler)
	logger2 := slog.New(handler)
	logger1.Info("message1")
	logger2.Info("message2")
}`,
			expect: `package main

import (
	"log/slog"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	NRhandler := nrslog.WrapHandler(NewRelicAgent, handler)
	logger1 := slog.New(NRhandler)
	logger2 := slog.New(NRhandler)
	logger1.Info("message1")
	logger2.Info("message2")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrslog.InstrumentSlogHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}
