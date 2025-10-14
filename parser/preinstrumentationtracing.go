package parser

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// DetectTransactions analyzes the AST to identify and track transactions within function declarations.
// It updates the transaction cache with function declarations and expressions related to transactions.
func DetectTransactions(manager *InstrumentationManager, c *dstutil.Cursor) {
	funcNode := c.Node()
	if decl, ok := funcNode.(*dst.FuncDecl); ok {
		manager.transactionCache.Functions[decl.Name.Name] = decl

		var currentTransaction string
		var recording bool
		dstutil.Apply(decl.Body, func(c *dstutil.Cursor) bool {
			node := c.Node()
			switch stmt := node.(type) {
			case *dst.AssignStmt:
				for _, rhs := range stmt.Rhs {
					if callExpr, ok := rhs.(*dst.CallExpr); ok {
						if selExpr, ok := callExpr.Fun.(*dst.SelectorExpr); ok {
							if _, ok := selExpr.X.(*dst.Ident); ok && selExpr.Sel.Name == "StartTransaction" {
								// Capture the transaction variable name
								if len(stmt.Lhs) > 0 {
									if txnVar, ok := stmt.Lhs[0].(*dst.Ident); ok {
										currentTransaction = txnVar.Name
										recording = true
									}
								}
							}
						}
					}
				}
			case *dst.ExprStmt:
				if recording {
					if callExpr, ok := stmt.X.(*dst.CallExpr); ok {
						manager.transactionCache.AddCall(currentTransaction, callExpr)
						if selExpr, ok := callExpr.Fun.(*dst.SelectorExpr); ok {
							if selExpr.Sel.Name == "End" && selExpr.X.(*dst.Ident).Name == currentTransaction {
								recording = false
								return false
							}
						}

						// Check if the transaction is passed to another function, if so track its calls
						for _, arg := range callExpr.Args {
							if ident, ok := arg.(*dst.Ident); ok && ident.Name == currentTransaction {
								if ident, ok := callExpr.Fun.(*dst.Ident); ok {
									if funcDecl, exists := manager.transactionCache.Functions[ident.Name]; exists {
										trackFunctionCalls(manager, funcDecl, currentTransaction)
									}
								}
							}
						}
					}
				}
			}
			return true
		}, nil)
	}
}

// trackFunctionCalls traverses the body of a function declaration to track expressions related to a transaction.
// It updates the transaction cache with expressions found within the function body.
func trackFunctionCalls(manager *InstrumentationManager, funcDecl *dst.FuncDecl, transactionName string) {
	// Traverse the function body to track calls
	dstutil.Apply(funcDecl.Body, func(c *dstutil.Cursor) bool {
		if callExpr, ok := c.Node().(*dst.CallExpr); ok {
			manager.transactionCache.AddCall(transactionName, callExpr)

			// Check if the call is an End method directly on the transaction
			if selExpr, ok := callExpr.Fun.(*dst.SelectorExpr); ok {
				if ident, ok := selExpr.X.(*dst.Ident); ok && selExpr.Sel.Name == "End" && ident.Name == transactionName {
					manager.transactionCache.TransactionState[transactionName] = true // Mark transaction as closed
					return false                                                      // Stop further traversal
				}
			}

			// Recursively track calls within functions that are called with the transaction
			if ident, ok := callExpr.Fun.(*dst.Ident); ok {
				if funcDecl, exists := manager.transactionCache.Functions[ident.Name]; exists {
					trackFunctionCalls(manager, funcDecl, transactionName)
				}
			}
		}
		return true
	}, nil)
}
