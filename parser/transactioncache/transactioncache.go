package transactioncache

import (
	"fmt"

	"github.com/dave/dst"
)

// TransactionData is responsible for maintaining metadata related to an individual transaction
// It contains the following components:
//
//   - Expressions: A list of dst.Expr statements active within the lifespan of the transaction
//
//   - IsClosed: A boolean to indicate whether a transaction is open or has ended, ensures that no
//     further expressions are added once a transaction is marked as ended
type TransactionData struct {
	Expressions []dst.Expr
	IsClosed    bool
}

// TransactionCache is responsible for tracking existing transactions within a Go application.
// It maintains the following components:
//
//   - Transactions: A map where each key is a transaction name, and the value is a pointer to a
//     TransactionData struct
//
//   - Functions: A map that stores already seen functions alongside their declarations. This is useful
//     for tracking transactions that span multiple function calls.
type TransactionCache struct {
	Transactions map[*dst.Ident]*TransactionData
	Functions    map[string]*dst.FuncDecl
}

func NewTransactionCache() *TransactionCache {
	return &TransactionCache{
		Transactions: make(map[*dst.Ident]*TransactionData),
		Functions:    make(map[string]*dst.FuncDecl),
	}
}

func NewTxnData() *TransactionData {
	return &TransactionData{
		Expressions: nil,
		IsClosed:    false,
	}
}

// IsTxnEnd returns true if a given dst.Expr is an `End()` operation for a given
// *dst.Ident transaction name, else false
func IsTxnEnd(txn *dst.Ident, expr dst.Expr) bool {
	if txn == nil || expr == nil {
		return false
	}

	callExpr, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}

	selExpr, ok := callExpr.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	if selExpr.Sel.Name != "End" {
		return false
	}

	ident, ok := selExpr.X.(*dst.Ident)
	if !ok || ident.Name != txn.Name {
		return false
	}

	return true
}

// SetClosed is a setter function for TransactionData to control the value of
// IsClosed. Returns true on success.
func (td *TransactionData) SetClosed(closed bool) bool {
	if td == nil {
		return false
	}
	td.IsClosed = closed
	return true
}

// AddExpr is a setter function for TransactionData to add an expression to the
// list of expressions. Returns true on success.
func (td *TransactionData) AddExpr(expr dst.Expr) bool {
	if td == nil || expr == nil {
		return false
	}
	td.Expressions = append(td.Expressions, expr)
	return true
}

// AddTxnToCache is a setter function for TransactionCache to add or update a
// TransactionData entry based on *dst.Ident key. Returns true on success.
func (tc *TransactionCache) AddTxnToCache(txnKey *dst.Ident, txnData *TransactionData) bool {
	if tc == nil || txnKey == nil || txnData == nil {
		return false
	}

	tc.Transactions[txnKey] = txnData
	return true
}

// AddCall adds an expression to the list of expressions associated with a transaction.
// It first checks if the transaction is closed, and if so, it does not add the expression.
// If the expression is an 'End' call directly on the transaction, it marks the transaction as closed.
// If the 'End' call is part of a segment [ex: defer txn.StartSegment.End()], it does not mark the transaction as closed.
func (tc *TransactionCache) AddCall(transaction *dst.Ident, expr dst.Expr) bool {
	if tc == nil || tc.Transactions == nil || transaction == nil || expr == nil {
		return false // Enforce initialization of TransactionCache
	}

	// Check if the transaction is closed
	txn, ok := tc.Transactions[transaction]
	if ok && txn.IsClosed {
		return false // Do not add calls to a closed transaction
	}

	var txnData *TransactionData
	if !ok {
		txnData = NewTxnData()
	} else {
		txnData = tc.Transactions[transaction]
	}

	// Check if the call is an End method directly on the transaction
	if IsTxnEnd(transaction, expr) {
		txnData.IsClosed = true
	}

	txnData.AddExpr(expr)

	return tc.AddTxnToCache(transaction, txnData)
}

// AddFuncDecl adds all expressions from a function declaration to a transaction.
// The transaction is marked as closed with the last element in the function body
// This handles cases where transaction start/end are obfuscated behind middleware.
func (tc *TransactionCache) AddFuncDecl(funcDecl *dst.FuncDecl) bool {
	if tc == nil || tc.Transactions == nil || funcDecl == nil {
		return false // Enforce initialization of TransactionCache
	}

	// Initialize a new transaction data object for fresh transactions
	txnData := NewTxnData()

	// Traverse all statements in the function body
	for _, stmt := range funcDecl.Body.List {
		// Consider only expression statements
		if exprStmt, ok := stmt.(*dst.ExprStmt); ok {
			expr := exprStmt.X
			// Add the expression to the transaction
			txnData.AddExpr(expr)
		}
	}
	txnData.SetClosed(true)
	// Add transaction data to cache
	return tc.AddTxnToCache(funcDecl.Name, txnData)

}

// IsFunctionInTransactionScope checks if a given function name is present within any transaction.
// It iterates over all transactions and their expressions, returning true if the function name is found.
func (tc *TransactionCache) IsFunctionInTransactionScope(functionName string) bool {
	if functionName == "" {
		return false
	}
	for _, txnData := range tc.Transactions {
		for _, expr := range txnData.Expressions {
			callExpr, ok := expr.(*dst.CallExpr)
			if !ok {
				continue
			}

			ident, ok := callExpr.Fun.(*dst.Ident)
			if ok && ident.Name == functionName {
				return true
			}

			selExpr, ok := callExpr.Fun.(*dst.SelectorExpr)
			if !ok {
				continue
			}

			ident, ok = selExpr.X.(*dst.Ident)
			if ok && ident.Name == functionName {
				return true
			}
		}
	}
	return false
}

// ExtractNames returns the transaction names and the corresponding expression names (For Testing)
func (tc *TransactionCache) ExtractNames() (transactionNames []string, expressionNames map[string][]string) {
	expressionNames = make(map[string][]string)

	// Iterate over transactions and gather names
	for txnKey, txnData := range tc.Transactions {
		transactionNames = append(transactionNames, txnKey.Name)

		for _, expr := range txnData.Expressions {
			switch e := expr.(type) {
			case *dst.CallExpr:
				if selExpr, ok := e.Fun.(*dst.SelectorExpr); ok {
					if ident, identOk := selExpr.X.(*dst.Ident); identOk {
						expressionNames[txnKey.Name] = append(expressionNames[txnKey.Name], fmt.Sprintf("%s.%s", ident.Name, selExpr.Sel.Name))
					} else {
						expressionNames[txnKey.Name] = append(expressionNames[txnKey.Name], selExpr.Sel.Name)
					}
				} else if ident, identOk := e.Fun.(*dst.Ident); identOk {
					expressionNames[txnKey.Name] = append(expressionNames[txnKey.Name], ident.Name)
				} else {
					expressionNames[txnKey.Name] = append(expressionNames[txnKey.Name], "Unknown")
				}
			default:
				continue
			}
		}
	}

	return transactionNames, expressionNames
}

// CheckTransactionExists returns true if the *dst.Ident transaction is already
// recorded in the cache, otherwise false.
func (tc *TransactionCache) CheckTransactionExists(transaction *dst.Ident) bool {
	_, ok := tc.Transactions[transaction]
	return ok
}

// Print outputs Debug printing of cache
func (tc *TransactionCache) Print() {
	for txnKey, txnData := range tc.Transactions {
		fmt.Printf("Transaction: %s\n", txnKey.Name)
		for _, expr := range txnData.Expressions {
			switch e := expr.(type) {
			case *dst.CallExpr:
				selExpr, ok := e.Fun.(*dst.SelectorExpr)
				if ok {
					ident, identOk := selExpr.X.(*dst.Ident)
					if identOk {
						fmt.Printf("  Call: %s.%s\n", ident.Name, selExpr.Sel.Name)
					} else {
						fmt.Printf("  Call: %s\n", selExpr.Sel.Name)
					}
				} else {
					ident, identOk := e.Fun.(*dst.Ident)
					if identOk {
						fmt.Printf("  Call: %s\n", ident.Name)
					} else {
						fmt.Printf("  Call: Unknown\n")
					}
				}
			default:
				fmt.Printf("  Expr: %T\n", expr)
			}
		}
	}
}
