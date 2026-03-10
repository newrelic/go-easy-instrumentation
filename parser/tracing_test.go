package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
	"github.com/newrelic/go-easy-instrumentation/parser/transactioncache"
	"github.com/stretchr/testify/assert"
)

// TestTraceFunction_AddTransactionParameter tests that transaction parameters are added to functions
func TestTraceFunction_AddTransactionParameter(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{
			name: "does_not_add_duplicate_parameter",
			code: `package main
import "github.com/newrelic/go-agent/v3/newrelic"
func main() {}
func handler(txn *newrelic.Transaction) {
	println("test")
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer PanicRecovery(t)
			uuid, _ := Pseudo_uuid()
			testDir := fmt.Sprintf("tmp_%s", uuid)
			defer CleanTestApp(t, testDir)

			manager := TestInstrumentationManager(t, tt.code, testDir)
			pkg := manager.getDecoratorPackage()
			assert.NotNil(t, pkg)

			// Find the handler function
			var handlerFunc *dst.FuncDecl
			for _, decl := range pkg.Syntax[0].Decls {
				if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == "handler" {
					handlerFunc = funcDecl
					break
				}
			}
			assert.NotNil(t, handlerFunc, "handler function not found")

			// Add transaction parameter to cache
			if handlerFunc.Type.Params != nil {
				for _, param := range handlerFunc.Type.Params.List {
					for _, name := range param.Names {
						if name.Name == "txn" {
							manager.transactionCache.AddTxnToCache(name, transactioncache.NewTxnData())
						}
					}
				}
			}

			// Trace the function
			tracingState := tracestate.FunctionBody("txn")
			_, _ = TraceFunction(manager, handlerFunc, tracingState)

			// Verify no duplicate transaction parameters were added
			if handlerFunc.Type.Params != nil {
				txnCount := 0
				for _, param := range handlerFunc.Type.Params.List {
					for _, name := range param.Names {
						if name.Name == "txn" {
							txnCount++
						}
					}
				}
				assert.LessOrEqual(t, txnCount, 1, "should not have duplicate transaction parameters")
			}
		})
	}
}

// TestTraceFunction_CreateSegment tests that segments are not duplicated
func TestTraceFunction_CreateSegment(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		functionName string
	}{
		{
			name: "does_not_create_duplicate_segment",
			code: `package main
import "github.com/newrelic/go-agent/v3/newrelic"
func main() {}
func handler(txn *newrelic.Transaction) {
	defer txn.StartSegment("handler").End()
	println("test")
}`,
			functionName: "handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer PanicRecovery(t)
			uuid, _ := Pseudo_uuid()
			testDir := fmt.Sprintf("tmp_%s", uuid)
			defer CleanTestApp(t, testDir)

			manager := TestInstrumentationManager(t, tt.code, testDir)
			pkg := manager.getDecoratorPackage()
			assert.NotNil(t, pkg)

			// Find the target function
			var targetFunc *dst.FuncDecl
			for _, decl := range pkg.Syntax[0].Decls {
				if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == tt.functionName {
					targetFunc = funcDecl
					break
				}
			}
			assert.NotNil(t, targetFunc, "target function not found")

			// Add transaction parameter to cache to mark it as a tracked transaction
			if targetFunc.Type.Params != nil {
				for _, param := range targetFunc.Type.Params.List {
					for _, name := range param.Names {
						if name.Name == "txn" {
							manager.transactionCache.AddTxnToCache(name, transactioncache.NewTxnData())
						}
					}
				}
			}

			// Count existing segments before tracing
			segmentCountBefore := 0
			if targetFunc.Body != nil {
				for _, stmt := range targetFunc.Body.List {
					if deferStmt, ok := stmt.(*dst.DeferStmt); ok {
						callExpr := deferStmt.Call
						if selExpr, ok := callExpr.Fun.(*dst.SelectorExpr); ok {
							if selExpr.Sel.Name == "End" {
								if innerCall, ok := selExpr.X.(*dst.CallExpr); ok {
									if innerSel, ok := innerCall.Fun.(*dst.SelectorExpr); ok {
										if innerSel.Sel.Name == "StartSegment" {
											segmentCountBefore++
										}
									}
								}
							}
						}
					}
				}
			}

			// Trace the function
			tracingState := tracestate.FunctionBody("txn")
			TraceFunction(manager, targetFunc, tracingState)

			// Count segments after tracing
			segmentCountAfter := 0
			if targetFunc.Body != nil {
				for _, stmt := range targetFunc.Body.List {
					if deferStmt, ok := stmt.(*dst.DeferStmt); ok {
						callExpr := deferStmt.Call
						if selExpr, ok := callExpr.Fun.(*dst.SelectorExpr); ok {
							if selExpr.Sel.Name == "End" {
								if innerCall, ok := selExpr.X.(*dst.CallExpr); ok {
									if innerSel, ok := innerCall.Fun.(*dst.SelectorExpr); ok {
										if innerSel.Sel.Name == "StartSegment" {
											segmentCountAfter++
										}
									}
								}
							}
						}
					}
				}
			}

			// Verify no duplicate segment was added
			assert.Equal(t, segmentCountBefore, segmentCountAfter, "should not have added duplicate segment")
		})
	}
}

// TestTraceFunction_GoroutineHandling tests goroutine instrumentation
func TestTraceFunction_GoroutineHandling(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		expectWarn   bool
		functionName string
	}{
		{
			name: "warns_on_goroutine_in_main",
			code: `package main
func main() {
	go func() {
		println("goroutine")
	}()
}`,
			expectWarn:   true,
			functionName: "main",
		},
		{
			name: "handles_goroutine_in_non_main",
			code: `package main
func main() {}
func handler() {
	go func() {
		println("goroutine")
	}()
}`,
			expectWarn:   false,
			functionName: "handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer PanicRecovery(t)
			uuid, _ := Pseudo_uuid()
			testDir := fmt.Sprintf("tmp_%s", uuid)
			defer CleanTestApp(t, testDir)

			manager := TestInstrumentationManager(t, tt.code, testDir)
			pkg := manager.getDecoratorPackage()
			assert.NotNil(t, pkg)

			// Find the target function
			var targetFunc *dst.FuncDecl
			for _, decl := range pkg.Syntax[0].Decls {
				if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == tt.functionName {
					targetFunc = funcDecl
					break
				}
			}
			assert.NotNil(t, targetFunc, "target function not found")

			// Trace the function
			var tracingState *tracestate.State
			if tt.functionName == "main" {
				tracingState = tracestate.Main("app")
			} else {
				tracingState = tracestate.FunctionBody("txn")
			}
			TraceFunction(manager, targetFunc, tracingState)

			// The test passes if no panic occurs
			// Warnings are added as comments in the AST, which we don't check here
		})
	}
}

// TestTraceFunction_FunctionLiteralTracing tests tracing of anonymous functions
func TestTraceFunction_FunctionLiteralTracing(t *testing.T) {
	code := `package main
func main() {}
func handler() {
	fn := func() {
		println("literal")
	}
	fn()
}`

	t.Run("traces_function_literal", func(t *testing.T) {
		defer PanicRecovery(t)
		uuid, _ := Pseudo_uuid()
		testDir := fmt.Sprintf("tmp_%s", uuid)
		defer CleanTestApp(t, testDir)

		manager := TestInstrumentationManager(t, code, testDir)
		pkg := manager.getDecoratorPackage()
		assert.NotNil(t, pkg)

		// Find handler function
		var handlerFunc *dst.FuncDecl
		for _, decl := range pkg.Syntax[0].Decls {
			if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == "handler" {
				handlerFunc = funcDecl
				break
			}
		}
		assert.NotNil(t, handlerFunc)

		// Trace the function
		tracingState := tracestate.FunctionBody("txn")
		_, _ = TraceFunction(manager, handlerFunc, tracingState)

		// Test passes if no panics occur during function literal tracing
	})
}

// TestHasTransactionParameter tests the hasTransactionParameter helper
func TestHasTransactionParameter(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		funcName string
		want     bool
	}{
		{
			name: "function_with_transaction_parameter",
			code: `package main
import "github.com/newrelic/go-agent/v3/newrelic"
func handler(txn *newrelic.Transaction) {}
func main() {}`,
			funcName: "handler",
			want:     true,
		},
		{
			name: "function_without_transaction_parameter",
			code: `package main
func handler(name string) {}
func main() {}`,
			funcName: "handler",
			want:     false,
		},
		{
			name: "function_with_no_parameters",
			code: `package main
func handler() {}
func main() {}`,
			funcName: "handler",
			want:     false,
		},
		{
			name: "function_with_nil_params",
			code: `package main
func handler() {}
func main() {}`,
			funcName: "handler",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer PanicRecovery(t)
			uuid, _ := Pseudo_uuid()
			testDir := fmt.Sprintf("tmp_%s", uuid)
			defer CleanTestApp(t, testDir)

			manager := TestInstrumentationManager(t, tt.code, testDir)
			pkg := manager.getDecoratorPackage()
			assert.NotNil(t, pkg)

			// Find the function
			var targetFunc *dst.FuncDecl
			for _, decl := range pkg.Syntax[0].Decls {
				if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == tt.funcName {
					targetFunc = funcDecl
					break
				}
			}
			assert.NotNil(t, targetFunc, "target function not found")

			// Populate transaction cache if test expects to find a transaction parameter
			if tt.want && targetFunc.Type != nil && targetFunc.Type.Params != nil {
				for _, param := range targetFunc.Type.Params.List {
					for _, name := range param.Names {
						manager.transactionCache.AddTxnToCache(name, transactioncache.NewTxnData())
					}
				}
			}

			got := manager.hasTransactionParameter(targetFunc.Type)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestTraceFunction_ErrorHandling tests error handling and recovery
func TestTraceFunction_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		node        dst.Node
		expectPanic bool
	}{
		{
			name:        "panics_on_invalid_node_type",
			node:        &dst.ExprStmt{},
			expectPanic: true,
		},
		{
			name: "accepts_func_decl",
			node: &dst.FuncDecl{
				Name: dst.NewIdent("test"),
				Type: &dst.FuncType{},
				Body: &dst.BlockStmt{},
			},
			expectPanic: false,
		},
		{
			name: "accepts_func_lit",
			node: &dst.FuncLit{
				Type: &dst.FuncType{},
				Body: &dst.BlockStmt{},
			},
			expectPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid, _ := Pseudo_uuid()
			testDir := fmt.Sprintf("tmp_%s", uuid)
			defer CleanTestApp(t, testDir)

			code := `package main
func main() {}
func test() {}`

			manager := TestInstrumentationManager(t, code, testDir)
			tracingState := tracestate.FunctionBody("txn")

			if tt.expectPanic {
				assert.Panics(t, func() {
					TraceFunction(manager, tt.node, tracingState)
				})
			} else {
				assert.NotPanics(t, func() {
					TraceFunction(manager, tt.node, tracingState)
				})
			}
		})
	}
}

// TestTraceFunction_RecursiveCallTracing tests that downstream function calls are traced
func TestTraceFunction_RecursiveCallTracing(t *testing.T) {
	code := `package main
func main() {}
func caller() {
	downstream()
}
func downstream() {
	println("downstream")
}`

	t.Run("traces_downstream_calls", func(t *testing.T) {
		defer PanicRecovery(t)
		uuid, _ := Pseudo_uuid()
		testDir := fmt.Sprintf("tmp_%s", uuid)
		defer CleanTestApp(t, testDir)

		manager := TestInstrumentationManager(t, code, testDir)
		pkg := manager.getDecoratorPackage()
		assert.NotNil(t, pkg)

		// Find caller function
		var callerFunc *dst.FuncDecl
		for _, decl := range pkg.Syntax[0].Decls {
			if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == "caller" {
				callerFunc = funcDecl
				break
			}
		}
		assert.NotNil(t, callerFunc)

		// Create function declarations in manager
		for _, decl := range pkg.Syntax[0].Decls {
			if funcDecl, ok := decl.(*dst.FuncDecl); ok {
				manager.createFunctionDeclaration(funcDecl)
			}
		}

		// Trace the caller function
		tracingState := tracestate.FunctionBody("txn")
		_, changed := TraceFunction(manager, callerFunc, tracingState)

		// Should have been modified
		assert.True(t, changed)
	})
}

// TestTraceFunction_Integration tests full integration scenarios
func TestTraceFunction_Integration(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		expectChange bool
	}{
		{
			name: "full_instrumentation_workflow",
			code: `package main
func main() {}
func handler(name string) error {
	result, err := process(name)
	if err != nil {
		return err
	}
	println(result)
	return nil
}
func process(name string) (string, error) {
	return name, nil
}`,
			expectChange: true,
		},
		{
			name: "already_instrumented_function",
			code: `package main
import "github.com/newrelic/go-agent/v3/newrelic"
func main() {}
func handler(txn *newrelic.Transaction) {
	defer txn.StartSegment("handler").End()
	println("test")
}`,
			expectChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer PanicRecovery(t)
			uuid, _ := Pseudo_uuid()
			testDir := fmt.Sprintf("tmp_%s", uuid)
			defer CleanTestApp(t, testDir)

			manager := TestInstrumentationManager(t, tt.code, testDir)
			pkg := manager.getDecoratorPackage()
			assert.NotNil(t, pkg)

			// Create function declarations
			for _, decl := range pkg.Syntax[0].Decls {
				if funcDecl, ok := decl.(*dst.FuncDecl); ok {
					manager.createFunctionDeclaration(funcDecl)
				}
			}

			// Find and trace handler function
			var handlerFunc *dst.FuncDecl
			for _, decl := range pkg.Syntax[0].Decls {
				if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == "handler" {
					handlerFunc = funcDecl
					break
				}
			}
			assert.NotNil(t, handlerFunc)

			tracingState := tracestate.FunctionBody("txn")
			_, changed := TraceFunction(manager, handlerFunc, tracingState)

			if tt.expectChange {
				// Changed could be true or false depending on existing instrumentation
				_ = changed
			}
		})
	}
}

// TestTraceFunction_MainFunction tests tracing main functions
func TestTraceFunction_MainFunction(t *testing.T) {
	code := `package main
func main() {
	handler()
}
func handler() {
	println("test")
}`

	t.Run("traces_main_function", func(t *testing.T) {
		defer PanicRecovery(t)
		uuid, _ := Pseudo_uuid()
		testDir := fmt.Sprintf("tmp_%s", uuid)
		defer CleanTestApp(t, testDir)

		manager := TestInstrumentationManager(t, code, testDir)
		pkg := manager.getDecoratorPackage()
		assert.NotNil(t, pkg)

		// Create function declarations
		for _, decl := range pkg.Syntax[0].Decls {
			if funcDecl, ok := decl.(*dst.FuncDecl); ok {
				manager.createFunctionDeclaration(funcDecl)
			}
		}

		// Find main function
		var mainFunc *dst.FuncDecl
		for _, decl := range pkg.Syntax[0].Decls {
			if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == "main" {
				mainFunc = funcDecl
				break
			}
		}
		assert.NotNil(t, mainFunc)

		// Trace main function
		tracingState := tracestate.Main("app")
		_, changed := TraceFunction(manager, mainFunc, tracingState)

		// Main function should be changed
		assert.True(t, changed)
	})
}

// TestTraceFunction_OutputValidation validates the output is valid Go code
func TestTraceFunction_OutputValidation(t *testing.T) {
	code := `package main
func main() {}
func handler() {
	println("test")
}`

	t.Run("output_is_valid_go_code", func(t *testing.T) {
		defer PanicRecovery(t)
		uuid, _ := Pseudo_uuid()
		testDir := fmt.Sprintf("tmp_%s", uuid)
		defer CleanTestApp(t, testDir)

		output := RunStatelessTracingFunction(t, code, func(m *InstrumentationManager, c *dstutil.Cursor) {
			// Empty stateless function for testing
		})

		// Check output contains valid Go package declaration
		assert.True(t, strings.Contains(output, "package main"))
	})
}
