package nrlogrus_test

import (
	"testing"

	"github.com/newrelic/go-easy-instrumentation/integrations/nrlogrus"
	"github.com/newrelic/go-easy-instrumentation/parser"

	"github.com/stretchr/testify/assert"
)

// instrumentationCase is one row of input → expected-output for the handler.
type instrumentationCase struct {
	name   string
	code   string
	expect string
}

// runCases runs each case through InstrumentLogrusHandler and asserts the
// formatted output matches.
func runCases(t *testing.T, cases []instrumentationCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrlogrus.InstrumentLogrusHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// ---------------------------------------------------------------------------
// Pattern 1: logger.SetFormatter(...) — wrap the arg in place
// ---------------------------------------------------------------------------

func TestInstrumentLogrusHandler_LoggerSetFormatter(t *testing.T) {
	runCases(t, []instrumentationCase{
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
		{
			name: "two_set_formatters_same_logger_both_wrapped",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetFormatter(&logrus.TextFormatter{})
	logger.Info("hi")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.JSONFormatter{}))
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logger.Info("hi")
}
`,
		},
	})
}

// ---------------------------------------------------------------------------
// Pattern 2: logger := logrus.New() with no SetFormatter — inject default
// ---------------------------------------------------------------------------

func TestInstrumentLogrusHandler_LoggerNoSetFormatter(t *testing.T) {
	runCases(t, []instrumentationCase{
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
		{
			name: "var_declared_logger",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	var logger = logrus.New()
	logger.Info("hi")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	var logger = logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logger.Info("hi")
}
`,
		},
		{
			name: "split_form_var_decl_then_assign",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	var logger *logrus.Logger
	logger = logrus.New()
	logger.Info("hi")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	var logger *logrus.Logger
	logger = logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logger.Info("hi")
}
`,
		},
		{
			name: "with_field_chain_does_not_break_pattern_2",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	logger := logrus.New()
	logger.WithField("k", "v").Info("hi")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logger.WithField("k", "v").Info("hi")
}
`,
		},
	})
}

// ---------------------------------------------------------------------------
// Pattern 3: logrus.SetFormatter(...) — wrap the arg in place
// ---------------------------------------------------------------------------

func TestInstrumentLogrusHandler_PackageSetFormatter(t *testing.T) {
	runCases(t, []instrumentationCase{
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
		{
			name: "package_set_formatter_no_subsequent_calls",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.JSONFormatter{}))
}
`,
		},
	})
}

// ---------------------------------------------------------------------------
// Pattern 4: standard logger only — inject logrus.SetFormatter before first call
// ---------------------------------------------------------------------------

func TestInstrumentLogrusHandler_StandardLoggerOnly(t *testing.T) {
	runCases(t, []instrumentationCase{
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
		{
			name: "non_logrus_stmt_before_first_call",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	x := 1
	_ = x
	logrus.Info("hello")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	x := 1
	_ = x
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logrus.Info("hello")
}
`,
		},
	})
}

// ---------------------------------------------------------------------------
// Mixed patterns — multiple loggers and combinations of patterns in one func
// ---------------------------------------------------------------------------

func TestInstrumentLogrusHandler_MixedPatterns(t *testing.T) {
	runCases(t, []instrumentationCase{
		{
			name: "logger_with_set_formatter_plus_logger_without",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	a := logrus.New()
	a.SetFormatter(&logrus.JSONFormatter{})
	b := logrus.New()
	b.Info("hi")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	a := logrus.New()
	a.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.JSONFormatter{}))
	b := logrus.New()
	b.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	b.Info("hi")
}
`,
		},
		{
			name: "logger_new_plus_package_set_formatter",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	logger := logrus.New()
	logrus.SetFormatter(&logrus.TextFormatter{})
	logger.Info("hi")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logger.Info("hi")
}
`,
		},
		{
			name: "set_formatter_cancels_only_its_logger",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	a := logrus.New()
	b := logrus.New()
	a.SetFormatter(&logrus.JSONFormatter{})
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
	b := logrus.New()
	b.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	a.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.JSONFormatter{}))
	a.Info("a")
	b.Info("b")
}
`,
		},
	})
}

// ---------------------------------------------------------------------------
// Idempotency — re-running on already-instrumented code is a no-op
// ---------------------------------------------------------------------------

func TestInstrumentLogrusHandler_Idempotent(t *testing.T) {
	runCases(t, []instrumentationCase{
		{
			name: "logger_set_formatter_already_wrapped",
			code: `package main
import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)
func main() {
	logger := logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.JSONFormatter{}))
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
			name: "package_set_formatter_already_wrapped",
			code: `package main
import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)
func main() {
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logrus.Info("hi")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logrus.Info("hi")
}
`,
		},
		{
			name: "mixed_one_already_wrapped_one_fresh",
			code: `package main
import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)
func main() {
	a := logrus.New()
	a.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.JSONFormatter{}))
	b := logrus.New()
	b.Info("hi")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	a := logrus.New()
	a.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.JSONFormatter{}))
	b := logrus.New()
	b.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	b.Info("hi")
}
`,
		},
	})
}

// ---------------------------------------------------------------------------
// Defensive — code shapes the handler must NOT misinterpret
// ---------------------------------------------------------------------------

func TestInstrumentLogrusHandler_Defensive(t *testing.T) {
	runCases(t, []instrumentationCase{
		{
			name: "no_logrus_usage_left_alone",
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
			name: "untracked_var_set_formatter_not_touched",
			code: `package main
import "github.com/sirupsen/logrus"
type fake struct{}
func (f *fake) SetFormatter(_ interface{}) {}
func main() {
	f := &fake{}
	f.SetFormatter(&logrus.JSONFormatter{})
	logrus.Info("hi")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

type fake struct{}

func (f *fake) SetFormatter(_ interface{}) {}
func main() {
	f := &fake{}
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	f.SetFormatter(&logrus.JSONFormatter{})
	logrus.Info("hi")
}
`,
		},
		{
			name: "set_formatter_with_no_args_left_alone",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	logger := logrus.New()
	logger.SetFormatter()
}`,
			expect: `package main

import "github.com/sirupsen/logrus"

func main() {
	logger := logrus.New()
	logger.SetFormatter()
}
`,
		},
		{
			name: "blank_assignment_does_not_track_underscore",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	_ = logrus.New()
	logrus.Info("hi")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	_ = logrus.New()
	logrus.Info("hi")
}
`,
		},
	})
}

// ---------------------------------------------------------------------------
// Aliased imports — the canonical path is matched, the alias is preserved
// ---------------------------------------------------------------------------

func TestInstrumentLogrusHandler_AliasedImport(t *testing.T) {
	runCases(t, []instrumentationCase{
		{
			name: "aliased_logrus_still_matched_and_alias_preserved",
			code: `package main
import lg "github.com/sirupsen/logrus"
func main() {
	logger := lg.New()
	logger.SetFormatter(&lg.JSONFormatter{})
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	lg "github.com/sirupsen/logrus"
)

func main() {
	logger := lg.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &lg.JSONFormatter{}))
}
`,
		},
	})
}

// ---------------------------------------------------------------------------
// Multiple functions — each is processed independently
// ---------------------------------------------------------------------------

func TestInstrumentLogrusHandler_MultipleFunctions(t *testing.T) {
	runCases(t, []instrumentationCase{
		{
			name: "two_funcs_each_pattern_independently_applied",
			code: `package main
import "github.com/sirupsen/logrus"
func helper() {
	logger := logrus.New()
	logger.Info("a")
}
func main() {
	helper()
	logrus.Info("b")
}`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func helper() {
	logger := logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logger.Info("a")
}
func main() {
	helper()
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logrus.Info("b")
}
`,
		},
		{
			name: "function_without_logrus_left_alone_alongside_one_with",
			code: `package main
import (
	"fmt"
	"github.com/sirupsen/logrus"
)
func untouched() {
	fmt.Println("nothing here")
}
func main() {
	logger := logrus.New()
	logger.Info("hi")
}`,
			expect: `package main

import (
	"fmt"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func untouched() {
	fmt.Println("nothing here")
}
func main() {
	logger := logrus.New()
	logger.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	logger.Info("hi")
}
`,
		},
	})
}

// ---------------------------------------------------------------------------
// Known limitations — these are documented, not aspirational
//
// The handler scans only the top-level statements of a function body. Loggers
// declared inside nested blocks (if/for/switch) and function literals are not
// detected. These tests pin the current behavior so future changes are visible.
// ---------------------------------------------------------------------------

func TestInstrumentLogrusHandler_KnownLimitations(t *testing.T) {
	runCases(t, []instrumentationCase{
		{
			name: "nested_logger_in_if_block_not_instrumented",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	cond := true
	if cond {
		logger := logrus.New()
		logger.Info("nested")
	}
}`,
			// Limitation: the nested logger is not detected, and pattern 4
			// fires because referencesLogrus walks the IfStmt recursively.
			// This injects a stray logrus.SetFormatter that does nothing for
			// the nested logger.
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	cond := true
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	if cond {
		logger := logrus.New()
		logger.Info("nested")
	}
}
`,
		},
		{
			name: "function_literal_not_instrumented",
			code: `package main
import "github.com/sirupsen/logrus"
func main() {
	f := func() {
		logger := logrus.New()
		logger.Info("from lit")
	}
	f()
}`,
			// Limitation: handler only acts on *dst.FuncDecl, so loggers inside
			// function literals pass through untouched. Pattern 4 fires on the
			// outer FuncDecl because referencesLogrus finds the nested call.
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(nrlogrus.NewFormatter(NewRelicAgent, &logrus.TextFormatter{}))
	f := func() {
		logger := logrus.New()
		logger.Info("from lit")
	}
	f()
}
`,
		},
	})
}
