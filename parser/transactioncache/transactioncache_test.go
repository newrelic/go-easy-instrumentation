package transactioncache

import (
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

// TestNewTransactionCache tests the cache constructor
func TestNewTransactionCache(t *testing.T) {
	tc := NewTransactionCache()

	assert.NotNil(t, tc)
	assert.NotNil(t, tc.Transactions)
	assert.NotNil(t, tc.Functions)
	assert.Equal(t, 0, len(tc.Transactions))
	assert.Equal(t, 0, len(tc.Functions))
}

// TestNewTxnData tests transaction data constructor
func TestNewTxnData(t *testing.T) {
	td := NewTxnData()

	assert.NotNil(t, td)
	assert.Nil(t, td.Expressions)
	assert.False(t, td.IsClosed)
}

// TestIsTxnEnd tests detection of transaction End() calls
func TestIsTxnEnd(t *testing.T) {
	txnIdent := dst.NewIdent("txn")

	tests := []struct {
		name string
		txn  *dst.Ident
		expr dst.Expr
		want bool
	}{
		{
			name: "detects_direct_end_call",
			txn:  txnIdent,
			expr: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("txn"),
					Sel: dst.NewIdent("End"),
				},
			},
			want: true,
		},
		{
			name: "rejects_chained_end_call",
			txn:  txnIdent,
			expr: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X: &dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   dst.NewIdent("txn"),
							Sel: dst.NewIdent("StartSegment"),
						},
					},
					Sel: dst.NewIdent("End"),
				},
			},
			want: false,
		},
		{
			name: "rejects_wrong_transaction_name",
			txn:  txnIdent,
			expr: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("otherTxn"),
					Sel: dst.NewIdent("End"),
				},
			},
			want: false,
		},
		{
			name: "rejects_wrong_method_name",
			txn:  txnIdent,
			expr: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("txn"),
					Sel: dst.NewIdent("Start"),
				},
			},
			want: false,
		},
		{
			name: "handles_nil_transaction",
			txn:  nil,
			expr: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("txn"),
					Sel: dst.NewIdent("End"),
				},
			},
			want: false,
		},
		{
			name: "handles_nil_expression",
			txn:  txnIdent,
			expr: nil,
			want: false,
		},
		{
			name: "rejects_non_call_expression",
			txn:  txnIdent,
			expr: dst.NewIdent("txn"),
			want: false,
		},
		{
			name: "rejects_call_without_selector",
			txn:  txnIdent,
			expr: &dst.CallExpr{
				Fun: dst.NewIdent("End"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTxnEnd(tt.txn, tt.expr)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestTransactionData_SetClosed tests the SetClosed method
func TestTransactionData_SetClosed(t *testing.T) {
	tests := []struct {
		name   string
		td     *TransactionData
		closed bool
		want   bool
	}{
		{
			name:   "sets_closed_to_true",
			td:     NewTxnData(),
			closed: true,
			want:   true,
		},
		{
			name:   "sets_closed_to_false",
			td:     NewTxnData(),
			closed: false,
			want:   true,
		},
		{
			name:   "handles_nil_transaction_data",
			td:     nil,
			closed: true,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.td.SetClosed(tt.closed)
			assert.Equal(t, tt.want, got)
			if tt.td != nil && got {
				assert.Equal(t, tt.closed, tt.td.IsClosed)
			}
		})
	}
}

// TestTransactionData_AddExpr tests adding expressions to transaction data
func TestTransactionData_AddExpr(t *testing.T) {
	tests := []struct {
		name string
		td   *TransactionData
		expr dst.Expr
		want bool
	}{
		{
			name: "adds_expression_successfully",
			td:   NewTxnData(),
			expr: dst.NewIdent("test"),
			want: true,
		},
		{
			name: "handles_nil_transaction_data",
			td:   nil,
			expr: dst.NewIdent("test"),
			want: false,
		},
		{
			name: "handles_nil_expression",
			td:   NewTxnData(),
			expr: nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.td.AddExpr(tt.expr)
			assert.Equal(t, tt.want, got)
			if tt.td != nil && tt.expr != nil && got {
				assert.Contains(t, tt.td.Expressions, tt.expr)
			}
		})
	}
}

// TestTransactionCache_AddTxnToCache tests adding transactions to cache
func TestTransactionCache_AddTxnToCache(t *testing.T) {
	tests := []struct {
		name    string
		tc      *TransactionCache
		txnKey  *dst.Ident
		txnData *TransactionData
		want    bool
	}{
		{
			name:    "adds_transaction_successfully",
			tc:      NewTransactionCache(),
			txnKey:  dst.NewIdent("txn"),
			txnData: NewTxnData(),
			want:    true,
		},
		{
			name:    "handles_nil_cache",
			tc:      nil,
			txnKey:  dst.NewIdent("txn"),
			txnData: NewTxnData(),
			want:    false,
		},
		{
			name:    "handles_nil_key",
			tc:      NewTransactionCache(),
			txnKey:  nil,
			txnData: NewTxnData(),
			want:    false,
		},
		{
			name:    "handles_nil_data",
			tc:      NewTransactionCache(),
			txnKey:  dst.NewIdent("txn"),
			txnData: nil,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tc.AddTxnToCache(tt.txnKey, tt.txnData)
			assert.Equal(t, tt.want, got)
			if tt.tc != nil && tt.txnKey != nil && tt.txnData != nil && got {
				assert.Equal(t, tt.txnData, tt.tc.Transactions[tt.txnKey])
			}
		})
	}
}

// TestTransactionCache_AddCall tests adding calls to transactions
func TestTransactionCache_AddCall(t *testing.T) {
	tests := []struct {
		name        string
		setupCache  func() *TransactionCache
		transaction *dst.Ident
		expr        dst.Expr
		want        bool
		checkClosed bool
		expectClose bool
	}{
		{
			name: "adds_call_to_open_transaction",
			setupCache: func() *TransactionCache {
				return NewTransactionCache()
			},
			transaction: dst.NewIdent("txn"),
			expr: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("txn"),
					Sel: dst.NewIdent("NoticeError"),
				},
			},
			want:        true,
			checkClosed: false,
		},
		{
			name: "rejects_call_to_closed_transaction",
			setupCache: func() *TransactionCache {
				tc := NewTransactionCache()
				txnKey := dst.NewIdent("txn")
				txnData := NewTxnData()
				txnData.IsClosed = true
				tc.AddTxnToCache(txnKey, txnData)
				return tc
			},
			transaction: dst.NewIdent("txn"),
			expr: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("txn"),
					Sel: dst.NewIdent("NoticeError"),
				},
			},
			want:        false,
			checkClosed: false,
		},
		{
			name: "detects_and_closes_on_end_call",
			setupCache: func() *TransactionCache {
				return NewTransactionCache()
			},
			transaction: dst.NewIdent("txn"),
			expr: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("txn"),
					Sel: dst.NewIdent("End"),
				},
			},
			want:        true,
			checkClosed: true,
			expectClose: true,
		},
		{
			name: "auto_creates_transaction_if_not_exists",
			setupCache: func() *TransactionCache {
				return NewTransactionCache()
			},
			transaction: dst.NewIdent("newTxn"),
			expr: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("newTxn"),
					Sel: dst.NewIdent("NoticeError"),
				},
			},
			want:        true,
			checkClosed: false,
		},
		{
			name: "handles_nil_cache",
			setupCache: func() *TransactionCache {
				return nil
			},
			transaction: dst.NewIdent("txn"),
			expr:        dst.NewIdent("test"),
			want:        false,
		},
		{
			name: "handles_nil_transaction",
			setupCache: func() *TransactionCache {
				return NewTransactionCache()
			},
			transaction: nil,
			expr:        dst.NewIdent("test"),
			want:        false,
		},
		{
			name: "handles_nil_expression",
			setupCache: func() *TransactionCache {
				return NewTransactionCache()
			},
			transaction: dst.NewIdent("txn"),
			expr:        nil,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := tt.setupCache()
			got := tc.AddCall(tt.transaction, tt.expr)
			assert.Equal(t, tt.want, got)

			if tt.checkClosed && tc != nil && tt.transaction != nil {
				txnData := tc.Transactions[tt.transaction]
				assert.NotNil(t, txnData)
				assert.Equal(t, tt.expectClose, txnData.IsClosed)
			}
		})
	}
}

// TestTransactionCache_AddFuncDecl tests adding function declarations
func TestTransactionCache_AddFuncDecl(t *testing.T) {
	tests := []struct {
		name     string
		tc       *TransactionCache
		funcDecl *dst.FuncDecl
		want     bool
	}{
		{
			name: "adds_function_declaration_successfully",
			tc:   NewTransactionCache(),
			funcDecl: &dst.FuncDecl{
				Name: dst.NewIdent("handler"),
				Body: &dst.BlockStmt{
					List: []dst.Stmt{
						&dst.ExprStmt{
							X: &dst.CallExpr{
								Fun: dst.NewIdent("test"),
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name:     "handles_nil_cache",
			tc:       nil,
			funcDecl: &dst.FuncDecl{Name: dst.NewIdent("test")},
			want:     false,
		},
		{
			name:     "handles_nil_function_declaration",
			tc:       NewTransactionCache(),
			funcDecl: nil,
			want:     false,
		},
		{
			name: "handles_function_with_no_body",
			tc:   NewTransactionCache(),
			funcDecl: &dst.FuncDecl{
				Name: dst.NewIdent("handler"),
				Body: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tc.AddFuncDecl(tt.funcDecl)
			assert.Equal(t, tt.want, got)

			if tt.tc != nil && tt.funcDecl != nil && tt.funcDecl.Body != nil && got {
				txnData := tt.tc.Transactions[tt.funcDecl.Name]
				assert.NotNil(t, txnData)
				assert.True(t, txnData.IsClosed)
			}
		})
	}
}

// TestTransactionCache_IsFunctionInTransactionScope tests function scope checking
func TestTransactionCache_IsFunctionInTransactionScope(t *testing.T) {
	tests := []struct {
		name         string
		setupCache   func() *TransactionCache
		functionName string
		want         bool
	}{
		{
			name: "finds_function_in_transaction_scope",
			setupCache: func() *TransactionCache {
				tc := NewTransactionCache()
				txnKey := dst.NewIdent("txn")
				txnData := NewTxnData()
				txnData.AddExpr(&dst.CallExpr{
					Fun: dst.NewIdent("testFunc"),
				})
				tc.AddTxnToCache(txnKey, txnData)
				return tc
			},
			functionName: "testFunc",
			want:         true,
		},
		{
			name: "finds_function_with_selector_expression",
			setupCache: func() *TransactionCache {
				tc := NewTransactionCache()
				txnKey := dst.NewIdent("txn")
				txnData := NewTxnData()
				txnData.AddExpr(&dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent("obj"),
						Sel: dst.NewIdent("Method"),
					},
				})
				tc.AddTxnToCache(txnKey, txnData)
				return tc
			},
			functionName: "obj",
			want:         true,
		},
		{
			name: "does_not_find_missing_function",
			setupCache: func() *TransactionCache {
				tc := NewTransactionCache()
				txnKey := dst.NewIdent("txn")
				txnData := NewTxnData()
				txnData.AddExpr(&dst.CallExpr{
					Fun: dst.NewIdent("otherFunc"),
				})
				tc.AddTxnToCache(txnKey, txnData)
				return tc
			},
			functionName: "testFunc",
			want:         false,
		},
		{
			name: "handles_empty_function_name",
			setupCache: func() *TransactionCache {
				return NewTransactionCache()
			},
			functionName: "",
			want:         false,
		},
		{
			name: "handles_non_call_expressions",
			setupCache: func() *TransactionCache {
				tc := NewTransactionCache()
				txnKey := dst.NewIdent("txn")
				txnData := NewTxnData()
				txnData.AddExpr(dst.NewIdent("notACall"))
				tc.AddTxnToCache(txnKey, txnData)
				return tc
			},
			functionName: "notACall",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := tt.setupCache()
			got := tc.IsFunctionInTransactionScope(tt.functionName)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestTransactionCache_CheckTransactionExists tests transaction existence checking
func TestTransactionCache_CheckTransactionExists(t *testing.T) {
	tests := []struct {
		name        string
		setupCache  func() *TransactionCache
		transaction *dst.Ident
		want        bool
	}{
		{
			name: "finds_existing_transaction",
			setupCache: func() *TransactionCache {
				tc := NewTransactionCache()
				txnKey := dst.NewIdent("txn")
				tc.AddTxnToCache(txnKey, NewTxnData())
				return tc
			},
			transaction: dst.NewIdent("txn"),
			want:        true,
		},
		{
			name: "does_not_find_missing_transaction",
			setupCache: func() *TransactionCache {
				return NewTransactionCache()
			},
			transaction: dst.NewIdent("missing"),
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := tt.setupCache()
			got := tc.CheckTransactionExists(tt.transaction)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestTransactionCache_ExtractNames tests the ExtractNames helper function
func TestTransactionCache_ExtractNames(t *testing.T) {
	t.Run("extracts_transaction_and_expression_names", func(t *testing.T) {
		tc := NewTransactionCache()
		txnKey := dst.NewIdent("txn")
		txnData := NewTxnData()

		// Add various expression types
		txnData.AddExpr(&dst.CallExpr{
			Fun: dst.NewIdent("simpleFunc"),
		})
		txnData.AddExpr(&dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent("obj"),
				Sel: dst.NewIdent("Method"),
			},
		})

		tc.AddTxnToCache(txnKey, txnData)

		txnNames, exprNames := tc.ExtractNames()

		assert.Contains(t, txnNames, "txn")
		assert.Contains(t, exprNames["txn"], "simpleFunc")
		assert.Contains(t, exprNames["txn"], "obj.Method")
	})

	t.Run("handles_empty_cache", func(t *testing.T) {
		tc := NewTransactionCache()
		txnNames, exprNames := tc.ExtractNames()

		assert.Equal(t, 0, len(txnNames))
		assert.NotNil(t, exprNames)
	})
}

// TestTransactionCache_Integration tests full workflow scenarios
func TestTransactionCache_Integration(t *testing.T) {
	t.Run("full_transaction_lifecycle", func(t *testing.T) {
		tc := NewTransactionCache()
		txnKey := dst.NewIdent("txn")

		// Start transaction
		startCall := &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent("app"),
				Sel: dst.NewIdent("StartTransaction"),
			},
		}
		assert.True(t, tc.AddCall(txnKey, startCall))

		// Add some operations
		noticeCall := &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent("txn"),
				Sel: dst.NewIdent("NoticeError"),
			},
		}
		assert.True(t, tc.AddCall(txnKey, noticeCall))

		// End transaction
		endCall := &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent("txn"),
				Sel: dst.NewIdent("End"),
			},
		}
		assert.True(t, tc.AddCall(txnKey, endCall))

		// Verify transaction is closed
		txnData := tc.Transactions[txnKey]
		assert.NotNil(t, txnData)
		assert.True(t, txnData.IsClosed)

		// Try to add to closed transaction
		moreCall := &dst.CallExpr{
			Fun: dst.NewIdent("moreStuff"),
		}
		assert.False(t, tc.AddCall(txnKey, moreCall))
	})

	t.Run("multiple_transactions", func(t *testing.T) {
		tc := NewTransactionCache()

		txn1 := dst.NewIdent("txn1")
		txn2 := dst.NewIdent("txn2")

		// Add calls to different transactions
		tc.AddCall(txn1, &dst.CallExpr{Fun: dst.NewIdent("func1")})
		tc.AddCall(txn2, &dst.CallExpr{Fun: dst.NewIdent("func2")})

		assert.Equal(t, 2, len(tc.Transactions))
		assert.True(t, tc.IsFunctionInTransactionScope("func1"))
		assert.True(t, tc.IsFunctionInTransactionScope("func2"))
	})
}
