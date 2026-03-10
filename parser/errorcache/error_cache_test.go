package errorcache

import (
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

// TestLoad tests loading error expression and statement
func TestLoad(t *testing.T) {
	tests := []struct {
		name      string
		errorexpr dst.Expr
		errorstmt dst.Stmt
	}{
		{
			name:      "loads_error_expression_and_statement",
			errorexpr: dst.NewIdent("err"),
			errorstmt: &dst.ExprStmt{X: dst.NewIdent("test")},
		},
		{
			name:      "loads_nil_expression_and_statement",
			errorexpr: nil,
			errorstmt: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := &ErrorCache{}
			ec.Load(tt.errorexpr, tt.errorstmt)

			assert.Equal(t, tt.errorexpr, ec.errorexpr)
			assert.Equal(t, tt.errorstmt, ec.errorstmt)
		})
	}
}

// TestLoadExistingErrors tests appending existing errors
func TestLoadExistingErrors(t *testing.T) {
	t.Run("appends_multiple_existing_errors", func(t *testing.T) {
		ec := &ErrorCache{}

		err1 := dst.NewIdent("err1")
		err2 := dst.NewIdent("err2")
		err3 := dst.NewIdent("err3")

		ec.LoadExistingErrors(err1)
		ec.LoadExistingErrors(err2)
		ec.LoadExistingErrors(err3)

		assert.Equal(t, 3, len(ec.ExistingErrors))
		assert.Contains(t, ec.ExistingErrors, err1)
		assert.Contains(t, ec.ExistingErrors, err2)
		assert.Contains(t, ec.ExistingErrors, err3)
	})

	t.Run("handles_nil_errors", func(t *testing.T) {
		ec := &ErrorCache{}
		ec.LoadExistingErrors(nil)

		assert.Equal(t, 1, len(ec.ExistingErrors))
		assert.Nil(t, ec.ExistingErrors[0])
	})
}

// TestIsExistingError tests existing error detection
func TestIsExistingError(t *testing.T) {
	tests := []struct {
		name          string
		setupCache    func() (*ErrorCache, dst.Expr)
		expectExisting bool
	}{
		{
			name: "finds_existing_error",
			setupCache: func() (*ErrorCache, dst.Expr) {
				ec := &ErrorCache{}
				err := dst.NewIdent("err")
				ec.LoadExistingErrors(err)
				return ec, err
			},
			expectExisting: true,
		},
		{
			name: "does_not_find_non_existing_error",
			setupCache: func() (*ErrorCache, dst.Expr) {
				ec := &ErrorCache{}
				ec.LoadExistingErrors(dst.NewIdent("err1"))
				return ec, dst.NewIdent("err2")
			},
			expectExisting: false,
		},
		{
			name: "handles_non_ident_expression",
			setupCache: func() (*ErrorCache, dst.Expr) {
				ec := &ErrorCache{}
				ec.LoadExistingErrors(dst.NewIdent("err"))
				testErr := &dst.SelectorExpr{
					X:   dst.NewIdent("obj"),
					Sel: dst.NewIdent("err"),
				}
				return ec, testErr
			},
			expectExisting: false,
		},
		{
			name: "handles_empty_cache",
			setupCache: func() (*ErrorCache, dst.Expr) {
				return &ErrorCache{}, dst.NewIdent("err")
			},
			expectExisting: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec, testError := tt.setupCache()
			got := ec.IsExistingError(testError)
			assert.Equal(t, tt.expectExisting, got)
		})
	}
}

// TestGetExpression tests retrieving loaded expression
func TestGetExpression(t *testing.T) {
	tests := []struct {
		name         string
		setupCache   func() *ErrorCache
		expectExpr   dst.Expr
	}{
		{
			name: "retrieves_loaded_expression",
			setupCache: func() *ErrorCache {
				ec := &ErrorCache{}
				expr := dst.NewIdent("err")
				ec.Load(expr, nil)
				return ec
			},
			expectExpr: dst.NewIdent("err"),
		},
		{
			name: "returns_nil_when_no_expression",
			setupCache: func() *ErrorCache {
				return &ErrorCache{}
			},
			expectExpr: nil,
		},
		{
			name: "returns_nil_after_clear",
			setupCache: func() *ErrorCache {
				ec := &ErrorCache{}
				ec.Load(dst.NewIdent("err"), nil)
				ec.Clear()
				return ec
			},
			expectExpr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := tt.setupCache()
			got := ec.GetExpression()

			if tt.expectExpr == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				// Compare identifier names
				if ident1, ok := got.(*dst.Ident); ok {
					if ident2, ok := tt.expectExpr.(*dst.Ident); ok {
						assert.Equal(t, ident2.Name, ident1.Name)
					}
				}
			}
		})
	}
}

// TestGetStatement tests retrieving loaded statement
func TestGetStatement(t *testing.T) {
	tests := []struct {
		name       string
		setupCache func() *ErrorCache
		expectStmt bool
	}{
		{
			name: "retrieves_loaded_statement",
			setupCache: func() *ErrorCache {
				ec := &ErrorCache{}
				stmt := &dst.ExprStmt{X: dst.NewIdent("test")}
				ec.Load(nil, stmt)
				return ec
			},
			expectStmt: true,
		},
		{
			name: "returns_nil_when_no_statement",
			setupCache: func() *ErrorCache {
				return &ErrorCache{}
			},
			expectStmt: false,
		},
		{
			name: "returns_nil_after_clear",
			setupCache: func() *ErrorCache {
				ec := &ErrorCache{}
				ec.Load(nil, &dst.ExprStmt{X: dst.NewIdent("test")})
				ec.Clear()
				return ec
			},
			expectStmt: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := tt.setupCache()
			got := ec.GetStatement()

			if tt.expectStmt {
				assert.NotNil(t, got)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

// TestClear tests clearing the error cache
func TestClear(t *testing.T) {
	t.Run("clears_expression_and_statement", func(t *testing.T) {
		ec := &ErrorCache{}
		ec.Load(dst.NewIdent("err"), &dst.ExprStmt{X: dst.NewIdent("test")})
		ec.LoadExistingErrors(dst.NewIdent("err1"))

		// Verify loaded
		assert.NotNil(t, ec.GetExpression())
		assert.NotNil(t, ec.GetStatement())

		// Clear
		ec.Clear()

		// Verify cleared
		assert.Nil(t, ec.GetExpression())
		assert.Nil(t, ec.GetStatement())
		// Note: ExistingErrors is not cleared by Clear()
		assert.Equal(t, 1, len(ec.ExistingErrors))
	})

	t.Run("clears_empty_cache", func(t *testing.T) {
		ec := &ErrorCache{}
		ec.Clear()

		assert.Nil(t, ec.GetExpression())
		assert.Nil(t, ec.GetStatement())
	})

	t.Run("can_load_after_clear", func(t *testing.T) {
		ec := &ErrorCache{}
		ec.Load(dst.NewIdent("err1"), nil)
		ec.Clear()
		ec.Load(dst.NewIdent("err2"), nil)

		expr := ec.GetExpression()
		assert.NotNil(t, expr)
		if ident, ok := expr.(*dst.Ident); ok {
			assert.Equal(t, "err2", ident.Name)
		}
	})
}

// TestExtractExistingErrors tests extracting error names
func TestExtractExistingErrors(t *testing.T) {
	tests := []struct {
		name       string
		setupCache func() *ErrorCache
		wantNames  []string
	}{
		{
			name: "extracts_single_error_name",
			setupCache: func() *ErrorCache {
				ec := &ErrorCache{}
				ec.LoadExistingErrors(dst.NewIdent("err"))
				return ec
			},
			wantNames: []string{"err"},
		},
		{
			name: "extracts_multiple_error_names",
			setupCache: func() *ErrorCache {
				ec := &ErrorCache{}
				ec.LoadExistingErrors(dst.NewIdent("err1"))
				ec.LoadExistingErrors(dst.NewIdent("err2"))
				ec.LoadExistingErrors(dst.NewIdent("err3"))
				return ec
			},
			wantNames: []string{"err1", "err2", "err3"},
		},
		{
			name: "handles_empty_cache",
			setupCache: func() *ErrorCache {
				return &ErrorCache{}
			},
			wantNames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := tt.setupCache()
			got := ec.ExtractExistingErrors()
			assert.Equal(t, tt.wantNames, got)
		})
	}
}

// TestErrorCache_Integration tests full workflow
func TestErrorCache_Integration(t *testing.T) {
	t.Run("full_error_cache_lifecycle", func(t *testing.T) {
		ec := &ErrorCache{}

		// Initially empty
		assert.Nil(t, ec.GetExpression())
		assert.Nil(t, ec.GetStatement())
		assert.Equal(t, 0, len(ec.ExistingErrors))

		// Load error
		errExpr := dst.NewIdent("err")
		errStmt := &dst.AssignStmt{
			Lhs: []dst.Expr{dst.NewIdent("err")},
			Rhs: []dst.Expr{dst.NewIdent("someFunc()")},
		}
		ec.Load(errExpr, errStmt)

		// Verify loaded
		assert.NotNil(t, ec.GetExpression())
		assert.NotNil(t, ec.GetStatement())

		// Load existing errors
		oldErr1 := dst.NewIdent("oldErr1")
		oldErr2 := dst.NewIdent("oldErr2")
		ec.LoadExistingErrors(oldErr1)
		ec.LoadExistingErrors(oldErr2)
		assert.Equal(t, 2, len(ec.ExistingErrors))

		// Check for existing error using same pointers
		assert.True(t, ec.IsExistingError(oldErr1))
		assert.False(t, ec.IsExistingError(dst.NewIdent("newErr")))

		// Extract names
		names := ec.ExtractExistingErrors()
		assert.Contains(t, names, "oldErr1")
		assert.Contains(t, names, "oldErr2")

		// Clear
		ec.Clear()
		assert.Nil(t, ec.GetExpression())
		assert.Nil(t, ec.GetStatement())
		// Existing errors remain
		assert.Equal(t, 2, len(ec.ExistingErrors))

		// Load new error after clear
		newErr := dst.NewIdent("newErr")
		ec.Load(newErr, nil)
		assert.NotNil(t, ec.GetExpression())
	})

	t.Run("multiple_load_operations", func(t *testing.T) {
		ec := &ErrorCache{}

		// Load first error
		err1 := dst.NewIdent("err1")
		stmt1 := &dst.ExprStmt{X: dst.NewIdent("stmt1")}
		ec.Load(err1, stmt1)

		// Load second error (overwrites first)
		err2 := dst.NewIdent("err2")
		stmt2 := &dst.ExprStmt{X: dst.NewIdent("stmt2")}
		ec.Load(err2, stmt2)

		// Should have second error
		expr := ec.GetExpression()
		if ident, ok := expr.(*dst.Ident); ok {
			assert.Equal(t, "err2", ident.Name)
		}
	})

	t.Run("concurrent_existing_errors", func(t *testing.T) {
		ec := &ErrorCache{}

		// Load multiple existing errors and save pointers
		errs := make([]*dst.Ident, 5)
		for i := 1; i <= 5; i++ {
			errs[i-1] = dst.NewIdent("err" + string(rune('0'+i)))
			ec.LoadExistingErrors(errs[i-1])
		}

		assert.Equal(t, 5, len(ec.ExistingErrors))

		// All should be detectable using the same pointers
		assert.True(t, ec.IsExistingError(errs[0]))
		assert.True(t, ec.IsExistingError(errs[2]))
		assert.True(t, ec.IsExistingError(errs[4]))
	})
}

// TestErrorCache_EdgeCases tests edge cases
func TestErrorCache_EdgeCases(t *testing.T) {
	t.Run("load_with_both_nil", func(t *testing.T) {
		ec := &ErrorCache{}
		ec.Load(nil, nil)

		assert.Nil(t, ec.GetExpression())
		assert.Nil(t, ec.GetStatement())
	})

	t.Run("clear_multiple_times", func(t *testing.T) {
		ec := &ErrorCache{}
		ec.Load(dst.NewIdent("err"), nil)

		ec.Clear()
		ec.Clear()
		ec.Clear()

		assert.Nil(t, ec.GetExpression())
	})

	t.Run("is_existing_error_with_same_pointer", func(t *testing.T) {
		ec := &ErrorCache{}
		err := dst.NewIdent("err")
		ec.LoadExistingErrors(err)

		// Should find the exact same pointer
		assert.True(t, ec.IsExistingError(err))
	})

	t.Run("extract_with_duplicate_names", func(t *testing.T) {
		ec := &ErrorCache{}
		ec.LoadExistingErrors(dst.NewIdent("err"))
		ec.LoadExistingErrors(dst.NewIdent("err"))
		ec.LoadExistingErrors(dst.NewIdent("err"))

		names := ec.ExtractExistingErrors()
		// Should have all three, even with duplicate names
		assert.Equal(t, 3, len(names))
		assert.Equal(t, "err", names[0])
		assert.Equal(t, "err", names[1])
		assert.Equal(t, "err", names[2])
	})
}
