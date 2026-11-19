package parser

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTraceFunction_Recursion guards against tracing recursing forever when a
// traced function reaches itself again, directly or through other functions.
// A function used to be marked as traced only after its whole body had been
// walked, so a recursive call started tracing it again from the top.
func TestTraceFunction_Recursion(t *testing.T) {
	tests := []struct {
		name string
		code string
		// want are fragments the instrumented source must contain.
		want []string
	}{
		{
			name: "direct recursion",
			code: `package main

func main() {}

func handler() {
	fact(5)
}

func fact(n int) int {
	if n <= 1 {
		return 1
	}
	return n * fact(n-1)
}
`,
			want: []string{
				"func fact(n int, nrTxn *newrelic.Transaction) int {",
				"fact(n-1, nrTxn)",
			},
		},
		{
			name: "mutual recursion",
			code: `package main

func main() {}

func handler() {
	isEven(10)
}

func isEven(n int) bool {
	if n == 0 {
		return true
	}
	return isOdd(n - 1)
}

func isOdd(n int) bool {
	if n == 0 {
		return false
	}
	return isEven(n - 1)
}
`,
			want: []string{
				"func isEven(n int, nrTxn *newrelic.Transaction) bool {",
				"func isOdd(n int, nrTxn *newrelic.Transaction) bool {",
				"isOdd(n-1, nrTxn)",
				"isEven(n-1, nrTxn)",
			},
		},
		{
			name: "recursion through a goroutine",
			code: `package main

func main() {}

func handler() {
	walk(3)
}

func walk(depth int) {
	if depth == 0 {
		return
	}
	go walk(depth - 1)
}
`,
			want: []string{
				"func walk(depth int, nrTxn *newrelic.Transaction) {",
				"go walk(depth-1, nrTxn.NewGoroutine())",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid, err := Pseudo_uuid()
			require.NoError(t, err)
			testDir := fmt.Sprintf("tmp_%s", uuid)
			defer CleanTestApp(t, testDir)

			manager := TestInstrumentationManager(t, tt.code, testDir)
			pkg := manager.getDecoratorPackage()
			require.NotNil(t, pkg)

			var handler *dst.FuncDecl
			for _, decl := range pkg.Syntax[0].Decls {
				if fn, ok := decl.(*dst.FuncDecl); ok {
					manager.createFunctionDeclaration(fn)
					if fn.Name.Name == "handler" {
						handler = fn
					}
				}
			}
			require.NotNil(t, handler)

			// Trace on another goroutine, so that a regression fails the test
			// instead of hanging the whole run.
			done := make(chan struct{})
			go func() {
				defer close(done)
				TraceFunction(manager, handler, tracestate.FunctionBody(codegen.DefaultTransactionVariable))
			}()

			select {
			case <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("tracing a recursive function did not finish; it is likely recursing forever")
			}

			var buf bytes.Buffer
			restorer := decorator.NewRestorerWithImports(testDir, createTestResolver(testDir))
			require.NoError(t, restorer.Fprint(&buf, pkg.Syntax[0]))
			out := buf.String()

			for _, fragment := range tt.want {
				assert.Contains(t, out, fragment)
			}
			if t.Failed() {
				t.Logf("instrumented source:\n%s", out)
			}
		})
	}
}
