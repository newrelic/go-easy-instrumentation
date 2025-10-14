package transactioncache

import (
	"fmt"

	"github.com/dave/dst"
)

// TransactionCache is responsible for tracking existing transactions within a Go application.
// It maintains the following components:
//
//   - Transactions: A map where each key is a transaction name, and the value is a list of expressions
//     that are active within the lifespan of the transaction.
//
//   - Functions: A map that stores already seen functions alongside their declarations. This is useful
//     for tracking transactions that span multiple function calls.
//
//   - TransactionState: A map that tracks whether a transaction is open or has ended, ensuring that
//     no further expressions are added once a transaction is marked as ended.
type TransactionCache struct {
	Transactions     map[*dst.Ident][]dst.Expr
	Functions        map[string]*dst.FuncDecl
	TransactionState map[*dst.Ident]bool // Track whether a transaction is closed
}

func NewTransactionCache() *TransactionCache {
	return &TransactionCache{
		Transactions:     make(map[*dst.Ident][]dst.Expr),
		Functions:        make(map[string]*dst.FuncDecl),
		TransactionState: make(map[*dst.Ident]bool),
	}
}

// AddCall adds an expression to the list of expressions associated with a transaction.
// It first checks if the transaction is closed, and if so, it does not add the expression.
// If the expression is an 'End' call directly on the transaction, it marks the transaction as closed.
// If the 'End' call is part of a segment [ex: defer txn.StartSegment.End()], it does not mark the transaction as closed.
func (tc *TransactionCache) AddCall(transaction *dst.Ident, expr dst.Expr) {
	if tc.Transactions == nil {
		tc.Transactions = make(map[*dst.Ident][]dst.Expr)
	}
	// Check if the transaction is closed
	if closed, exists := tc.TransactionState[transaction]; exists && closed {
		return // Do not add calls to a closed transaction
	}
	// Check if the call is an End method directly on the transaction
	if callExpr, ok := expr.(*dst.CallExpr); ok {
		if selExpr, ok := callExpr.Fun.(*dst.SelectorExpr); ok {
			if selExpr.Sel.Name == "End" {
				// Check if the End method is called directly on the transaction
				if ident, ok := selExpr.X.(*dst.Ident); ok && ident.Name == transaction.Name {
					tc.TransactionState[transaction] = true // Mark transaction as closed
				} else if selExpr.X.(*dst.CallExpr) != nil {
					// This is likely part of a segment operation, do not mark transaction as closed
					return
				}
			}
		}
	}
	tc.Transactions[transaction] = append(tc.Transactions[transaction], expr)

}

// IsFunctionInTransactionScope checks if a given function name is present within any transaction.
// It iterates over all transactions and their expressions, returning true if the function name is found.
func (tc *TransactionCache) IsFunctionInTransactionScope(functionName string) bool {
	for _, exprs := range tc.Transactions {
		for _, expr := range exprs {
			if callExpr, ok := expr.(*dst.CallExpr); ok {
				if ident, ok := callExpr.Fun.(*dst.Ident); ok && ident.Name == functionName {
					return true
				}
				if selExpr, ok := callExpr.Fun.(*dst.SelectorExpr); ok {
					if ident, ok := selExpr.X.(*dst.Ident); ok && ident.Name == functionName {
						return true
					}
				}
			}
		}
	}
	return false
}

// ExtractNames returns the transaction names and the corresponding expression names (For Testing)
func (tc *TransactionCache) ExtractNames() (transactionNames []string, expressionNames map[string][]string) {
	expressionNames = make(map[string][]string)

	// Iterate over transactions and gather names
	for txn, exprs := range tc.Transactions {
		transactionNames = append(transactionNames, txn.Name)

		for _, expr := range exprs {
			switch e := expr.(type) {
			case *dst.CallExpr:
				if selExpr, ok := e.Fun.(*dst.SelectorExpr); ok {
					if ident, identOk := selExpr.X.(*dst.Ident); identOk {
						expressionNames[txn.Name] = append(expressionNames[txn.Name], fmt.Sprintf("%s.%s", ident.Name, selExpr.Sel.Name))
					} else {
						expressionNames[txn.Name] = append(expressionNames[txn.Name], selExpr.Sel.Name)
					}
				} else if ident, identOk := e.Fun.(*dst.Ident); identOk {
					expressionNames[txn.Name] = append(expressionNames[txn.Name], ident.Name)
				} else {
					expressionNames[txn.Name] = append(expressionNames[txn.Name], "Unknown")
				}
			default:
				continue
			}
		}
	}

	return transactionNames, expressionNames
}
func (tc *TransactionCache) CheckTransactionExists(transaction *dst.Ident) bool {
	for txn := range tc.Transactions {
		if txn.Name == transaction.Name {
			return true
		}
	}
	return false
}

// Debug printing of cache
func (tc *TransactionCache) Print() {
	for txn, exprs := range tc.Transactions {
		fmt.Printf("Transaction: %s\n", txn.Name)
		for _, expr := range exprs {
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
