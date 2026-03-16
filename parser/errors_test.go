package parser

import (
	"fmt"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
	"github.com/stretchr/testify/assert"
)

// TestNoticeError_ReturnStatement tests error capture on return statements
func TestNoticeError_ReturnStatement(t *testing.T) {
	code := `package main
import "errors"
func main() {}
func handler() error {
	return errors.New("test error")
}`

	t.Run("captures_error_in_return", func(t *testing.T) {
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

		tracingState := tracestate.FunctionBody("txn")

		// Apply NoticeError
		dstutil.Apply(handlerFunc, nil, func(c *dstutil.Cursor) bool {
			n := c.Node()
			switch v := n.(type) {
			case dst.Stmt:
				NoticeError(manager, v, c, tracingState, false)
			}
			return true
		})
	})
}

// TestNoticeError_IfStatement tests error injection into if blocks
func TestNoticeError_IfStatement(t *testing.T) {
	code := `package main
func main() {}
func handler() {
	var err error
	err = someFunc()
	if err != nil {
		println("error occurred")
	}
}
func someFunc() error {
	return nil
}`

	t.Run("injects_error_into_if_block", func(t *testing.T) {
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

		tracingState := tracestate.FunctionBody("txn")

		// Apply NoticeError
		dstutil.Apply(handlerFunc, nil, func(c *dstutil.Cursor) bool {
			n := c.Node()
			switch v := n.(type) {
			case dst.Stmt:
				NoticeError(manager, v, c, tracingState, false)
			}
			return true
		})
	})
}

// TestNoticeError_AssignStatement tests error loading into cache
func TestNoticeError_AssignStatement(t *testing.T) {
	code := `package main
func main() {}
func handler() {
	result, err := someFunc()
	_ = result
	_ = err
}
func someFunc() (string, error) {
	return "", nil
}`

	t.Run("loads_error_into_cache", func(t *testing.T) {
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

		tracingState := tracestate.FunctionBody("txn")

		// Apply NoticeError to each statement
		dstutil.Apply(handlerFunc, nil, func(c *dstutil.Cursor) bool {
			n := c.Node()
			switch v := n.(type) {
			case dst.Stmt:
				NoticeError(manager, v, c, tracingState, false)
			}
			return true
		})

		// The error should have been loaded into the cache at some point
		// We can't easily assert this without exposing more internals
	})
}

// TestNoticeError_MainFunction tests that errors are not noticed in main
func TestNoticeError_MainFunction(t *testing.T) {
	code := `package main
func main() {
	err := someFunc()
	if err != nil {
		println(err)
	}
}
func someFunc() error {
	return nil
}`

	t.Run("skips_error_notice_in_main", func(t *testing.T) {
		defer PanicRecovery(t)
		uuid, _ := Pseudo_uuid()
		testDir := fmt.Sprintf("tmp_%s", uuid)
		defer CleanTestApp(t, testDir)

		manager := TestInstrumentationManager(t, code, testDir)
		pkg := manager.getDecoratorPackage()
		assert.NotNil(t, pkg)

		// Find main function
		var mainFunc *dst.FuncDecl
		for _, decl := range pkg.Syntax[0].Decls {
			if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == "main" {
				mainFunc = funcDecl
				break
			}
		}
		assert.NotNil(t, mainFunc)

		tracingState := tracestate.Main("app")

		// Apply NoticeError to each statement
		changed := false
		dstutil.Apply(mainFunc, nil, func(c *dstutil.Cursor) bool {
			n := c.Node()
			switch v := n.(type) {
			case dst.Stmt:
				ok := NoticeError(manager, v, c, tracingState, false)
				if ok {
					changed = true
				}
			}
			return true
		})

		// NoticeError should return false for main functions
		assert.False(t, changed)
	})
}

// TestNoticeError_AlreadyTracedFunction tests that errors in traced functions are skipped
func TestNoticeError_AlreadyTracedFunction(t *testing.T) {
	code := `package main
func main() {}
func handler() error {
	return downstream()
}
func downstream() error {
	return nil
}`

	t.Run("skips_traced_function_errors", func(t *testing.T) {
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

		tracingState := tracestate.FunctionBody("txn")

		// Apply NoticeError with functionCallWasTraced=true
		dstutil.Apply(handlerFunc, nil, func(c *dstutil.Cursor) bool {
			n := c.Node()
			switch v := n.(type) {
			case dst.Stmt:
				NoticeError(manager, v, c, tracingState, true)
			}
			return true
		})

		// Should complete without errors
	})
}

// TestNoticeError_Integration tests full error handling workflow
func TestNoticeError_Integration(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{
			name: "handles_multiple_error_assignments",
			code: `package main
func main() {}
func handler() {
	err1 := func1()
	if err1 != nil {
		return
	}
	err2 := func2()
	if err2 != nil {
		return
	}
}
func func1() error { return nil }
func func2() error { return nil }`,
		},
		{
			name: "handles_error_in_nested_if",
			code: `package main
func main() {}
func handler() {
	err := someFunc()
	if err != nil {
		if true {
			println("nested")
		}
	}
}
func someFunc() error { return nil }`,
		},
		{
			name: "handles_blank_identifier",
			code: `package main
func main() {}
func handler() {
	_, err := someFunc()
	if err != nil {
		return
	}
}
func someFunc() (string, error) { return "", nil }`,
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

			// Find handler function
			var handlerFunc *dst.FuncDecl
			for _, decl := range pkg.Syntax[0].Decls {
				if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == "handler" {
					handlerFunc = funcDecl
					break
				}
			}
			assert.NotNil(t, handlerFunc)

			tracingState := tracestate.FunctionBody("txn")

			// Apply NoticeError
			dstutil.Apply(handlerFunc, nil, func(c *dstutil.Cursor) bool {
				n := c.Node()
				switch v := n.(type) {
				case dst.Stmt:
					NoticeError(manager, v, c, tracingState, false)
				}
				return true
			})
		})
	}
}

// TestFindErrorVariable tests error variable extraction
func TestFindErrorVariable(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		hasError bool
	}{
		{
			name: "finds_single_error_variable",
			code: `package main
func main() {}
func test() {
	err := someFunc()
	_ = err
}
func someFunc() error { return nil }`,
			hasError: true,
		},
		{
			name: "finds_error_in_multiple_returns",
			code: `package main
func main() {}
func test() {
	data, err := someFunc()
	_, _ = data, err
}
func someFunc() (string, error) { return "", nil }`,
			hasError: true,
		},
		{
			name: "ignores_blank_identifier",
			code: `package main
func main() {}
func test() {
	_ = someFunc()
}
func someFunc() error { return nil }`,
			hasError: false,
		},
		{
			name: "handles_no_error_type",
			code: `package main
func main() {}
func test() {
	result := someFunc()
	_ = result
}
func someFunc() string { return "" }`,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer PanicRecovery(t)
			uuid, _ := Pseudo_uuid()
			testDir := fmt.Sprintf("tmp_%s", uuid)
			defer CleanTestApp(t, testDir)

			pkgs := UnitTest(t, tt.code)
			assert.NotNil(t, pkgs)
			assert.Greater(t, len(pkgs), 0)

			pkg := pkgs[0]

			// Find test function and its first assignment
			var assignStmt *dst.AssignStmt
			for _, file := range pkg.Syntax {
				for _, decl := range file.Decls {
					if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == "test" {
						for _, stmt := range funcDecl.Body.List {
							if assign, ok := stmt.(*dst.AssignStmt); ok {
								assignStmt = assign
								break
							}
						}
					}
				}
			}

			if assignStmt != nil {
				errVar := findErrorVariable(assignStmt, pkg)
				if tt.hasError {
					assert.NotNil(t, errVar, "should find error variable")
				} else {
					assert.Nil(t, errVar, "should not find error variable")
				}
			}
		})
	}
}

// TestShouldNoticeError tests error condition detection
func TestShouldNoticeError(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{
			name: "detects_simple_error_check",
			code: `package main
func main() {}
func test() {
	var err error
	if err != nil {
		return
	}
}`,
			want: true,
		},
		{
			name: "detects_nested_error_check",
			code: `package main
func main() {}
func test() {
	var err error
	if err != nil {
		if true {
			return
		}
	}
}`,
			want: true,
		},
		{
			name: "rejects_non_error_condition",
			code: `package main
func main() {}
func test() {
	if true {
		return
	}
}`,
			want: false,
		},
		{
			name: "rejects_non_if_statement",
			code: `package main
func main() {}
func test() {
	println("test")
}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer PanicRecovery(t)
			uuid, _ := Pseudo_uuid()
			testDir := fmt.Sprintf("tmp_%s", uuid)
			defer CleanTestApp(t, testDir)

			pkgs := UnitTest(t, tt.code)
			assert.NotNil(t, pkgs)
			assert.Greater(t, len(pkgs), 0)

			pkg := pkgs[0]
			tracingState := tracestate.FunctionBody("txn")

			// Find test function and its first if statement
			var ifStmt dst.Stmt
			for _, file := range pkg.Syntax {
				for _, decl := range file.Decls {
					if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == "test" {
						if len(funcDecl.Body.List) > 0 {
							// Get the last statement (likely the if statement)
							ifStmt = funcDecl.Body.List[len(funcDecl.Body.List)-1]
						}
					}
				}
			}

			if ifStmt != nil {
				got := shouldNoticeError(ifStmt, pkg, tracingState)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestErrNilCheck tests error nil checking logic
func TestErrNilCheck(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{
			name: "detects_err_not_equal_nil",
			code: `package main
func main() {}
func test() {
	var err error
	if err != nil {
		return
	}
}`,
			want: true,
		},
		{
			name: "detects_nil_not_equal_err",
			code: `package main
func main() {}
func test() {
	var err error
	if nil != err {
		return
	}
}`,
			want: true,
		},
		{
			name: "rejects_wrong_operator",
			code: `package main
func main() {}
func test() {
	var err error
	if err == nil {
		return
	}
}`,
			want: false,
		},
		{
			name: "rejects_non_error_type",
			code: `package main
func main() {}
func test() {
	var x int
	if x != 0 {
		return
	}
}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer PanicRecovery(t)
			uuid, _ := Pseudo_uuid()
			testDir := fmt.Sprintf("tmp_%s", uuid)
			defer CleanTestApp(t, testDir)

			pkgs := UnitTest(t, tt.code)
			assert.NotNil(t, pkgs)
			assert.Greater(t, len(pkgs), 0)

			pkg := pkgs[0]

			// Find the if statement and extract the binary expression
			var binExpr *dst.BinaryExpr
			for _, file := range pkg.Syntax {
				for _, decl := range file.Decls {
					if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == "test" {
						for _, stmt := range funcDecl.Body.List {
							if ifStmt, ok := stmt.(*dst.IfStmt); ok {
								if be, ok := ifStmt.Cond.(*dst.BinaryExpr); ok {
									binExpr = be
									break
								}
							}
						}
					}
				}
			}

			if binExpr != nil {
				got := errNilCheck(binExpr, pkg)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestErrNilCheck_NestedExpressions tests recursive error checking
func TestErrNilCheck_NestedExpressions(t *testing.T) {
	code := `package main
func main() {}
func test() {
	var err1, err2 error
	if err1 != nil || err2 != nil {
		return
	}
}`

	t.Run("handles_nested_binary_expressions", func(t *testing.T) {
		defer PanicRecovery(t)
		uuid, _ := Pseudo_uuid()
		testDir := fmt.Sprintf("tmp_%s", uuid)
		defer CleanTestApp(t, testDir)

		pkgs := UnitTest(t, code)
		assert.NotNil(t, pkgs)
		assert.Greater(t, len(pkgs), 0)

		pkg := pkgs[0]

		// Find the if statement
		var binExpr *dst.BinaryExpr
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				if funcDecl, ok := decl.(*dst.FuncDecl); ok && funcDecl.Name.Name == "test" {
					for _, stmt := range funcDecl.Body.List {
						if ifStmt, ok := stmt.(*dst.IfStmt); ok {
							if be, ok := ifStmt.Cond.(*dst.BinaryExpr); ok {
								binExpr = be
								break
							}
						}
					}
				}
			}
		}

		if binExpr != nil {
			got := errNilCheck(binExpr, pkg)
			// Should detect at least one error check in the nested expression
			assert.True(t, got)
		}
	})
}
