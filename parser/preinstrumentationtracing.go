package parser

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/parser/transactioncache"
)

// DetectTransactions analyzes the AST to identify and track transactions within function declarations.
// It updates the transaction cache with function declarations and expressions related to transactions.
// If a function already has transactions, it marks the function as traced to prevent duplicate instrumentation.
func DetectTransactions(manager *InstrumentationManager, c *dstutil.Cursor) {
	funcNode := c.Node()
	if decl, ok := funcNode.(*dst.FuncDecl); ok {
		manager.transactionCache.Functions[decl.Name.Name] = decl

		var currentTransaction *dst.Ident
		hasExistingTransaction := false

		dstutil.Apply(decl.Body, func(c *dstutil.Cursor) bool {
			node := c.Node()
			switch stmt := node.(type) {
			case *dst.AssignStmt:
				for _, rhs := range stmt.Rhs {
					callExpr, ok := rhs.(*dst.CallExpr)
					if !ok {
						continue
					}
					selExpr, ok := callExpr.Fun.(*dst.SelectorExpr)
					if !ok {
						// Check if the callExpr is ident and if so, we should check to see if it's from context
						if ident, ok := callExpr.Fun.(*dst.Ident); ok {
							if ident.Name == "FromContext" {
								// Capture the transaction variable name
								if len(stmt.Lhs) > 0 {
									txnVar, ok := stmt.Lhs[0].(*dst.Ident)
									if ok && txnVar != nil {
										currentTransaction = txnVar
										hasExistingTransaction = true
									}
								}
							}
						}
						continue
					}
					_, ok = selExpr.X.(*dst.Ident)
					if ok && selExpr.Sel.Name == "StartTransaction" {
						// Capture the transaction variable name
						if len(stmt.Lhs) > 0 {
							txnVar, ok := stmt.Lhs[0].(*dst.Ident)
							if ok && txnVar != nil {
								currentTransaction = txnVar
								hasExistingTransaction = true
							}
						}
					}

				}
			case *dst.ExprStmt:
				if currentTransaction != nil {
					callExpr, ok := stmt.X.(*dst.CallExpr)
					if !ok {
						break
					}
					manager.transactionCache.AddCall(currentTransaction, callExpr)
					selExpr, ok := callExpr.Fun.(*dst.SelectorExpr)
					if ok {
						if selExpr.Sel.Name == "End" && selExpr.X.(*dst.Ident) == currentTransaction {
							return false
						}
					}
					// Check if the transaction is passed to another function, if so track its calls
					for _, arg := range callExpr.Args {
						ident, ok := arg.(*dst.Ident)
						if !ok || ident.Name != currentTransaction.Name {
							continue
						}
						ident, ok = callExpr.Fun.(*dst.Ident)
						if !ok {
							continue
						}
						funcDecl, exists := manager.transactionCache.Functions[ident.Name]
						if exists {
							trackFunctionCalls(manager, funcDecl, currentTransaction)
						}
					}
				}
			}
			return true
		}, nil)

		// If we found an existing transaction, mark this function as already traced
		if hasExistingTransaction {
			manager.createFunctionDeclaration(decl)
			manager.updateFunctionDeclaration(decl)
		}
	}
}

// trackFunctionCalls traverses the body of a function declaration to track expressions related to a transaction.
// It updates the transaction cache with expressions found within the function body.
func trackFunctionCalls(manager *InstrumentationManager, funcDecl *dst.FuncDecl, txn *dst.Ident) {
	// Traverse the function body to track calls
	dstutil.Apply(funcDecl.Body, func(c *dstutil.Cursor) bool {
		callExpr, ok := c.Node().(*dst.CallExpr)
		if !ok {
			return true
		}

		// Validate that we are able to add calls to the cache. Fail and bail if we are not.
		if !manager.transactionCache.AddCall(txn, callExpr) {
			return false
		}

		// Check if the call is an End method directly on the transaction
		if transactioncache.IsTxnEnd(txn, callExpr) {
			txnData, ok := manager.transactionCache.Transactions[txn]
			if !ok {
				return false
			}
			txnData.SetClosed(true)
			return false // Stop further traversal
		}

		// Recursively track calls within functions that are called with the transaction
		if ident, ok := callExpr.Fun.(*dst.Ident); ok {
			if funcDecl, exists := manager.transactionCache.Functions[ident.Name]; exists {
				trackFunctionCalls(manager, funcDecl, txn)
			}
		}
		return true
	}, nil)
}

func DetectErrors(manager *InstrumentationManager, c *dstutil.Cursor) {
	txns := manager.transactionCache.Transactions
	for _, txnData := range txns {
		// Check existing transactions to see if any have NoticeError calls
		for _, call := range txnData.Expressions {
			call, ok := call.(*dst.CallExpr)
			if !ok {
				return
			}
			funcCall, ok := call.Fun.(*dst.SelectorExpr)
			if ok && funcCall.Sel.Name == "NoticeError" {
				if len(call.Args) > 0 {
					if errVar, ok := call.Args[0].(*dst.Ident); ok {
						manager.errorCache.LoadExistingErrors(errVar)
					}
				}

			}

		}
	}
}

// DetectGinInstrumentation analyzes the AST to identify existing Gin instrumentation.
// It detects functions that already have nrgin.Transaction() calls and marks them as traced
// to prevent duplicate instrumentation.
func DetectGinInstrumentation(manager *InstrumentationManager, c *dstutil.Cursor) {
	funcNode := c.Node()
	if decl, ok := funcNode.(*dst.FuncDecl); ok {
		manager.transactionCache.Functions[decl.Name.Name] = decl

		// Traverse the function body to look for existing Gin instrumentation patterns
		hasGinTransaction := false
		dstutil.Apply(decl.Body, func(c *dstutil.Cursor) bool {
			node := c.Node()
			switch stmt := node.(type) {
			case *dst.AssignStmt:
				for _, rhs := range stmt.Rhs {
					callExpr, ok := rhs.(*dst.CallExpr)
					if !ok {
						continue
					}
					// Check for nrgin.Transaction(c) pattern
					if ident, ok := callExpr.Fun.(*dst.Ident); ok {
						if ident.Name == "Transaction" && ident.Path == "github.com/newrelic/go-agent/v3/integrations/nrgin" {
							hasGinTransaction = true
							// Capture the transaction variable name
							if len(stmt.Lhs) > 0 {
								if txnVar, ok := stmt.Lhs[0].(*dst.Ident); ok && txnVar != nil {
									txnData := transactioncache.NewTxnData()
									manager.transactionCache.AddTxnToCache(txnVar, txnData)
								}
							}
						}
					}
				}
			case *dst.ExprStmt:
				// Check for router.Use(nrgin.Middleware(...)) pattern
				if callExpr, ok := stmt.X.(*dst.CallExpr); ok {
					if selExpr, ok := callExpr.Fun.(*dst.SelectorExpr); ok {
						if selExpr.Sel.Name == "Use" {
							// Check if the argument is nrgin.Middleware
							if len(callExpr.Args) > 0 {
								if argCall, ok := callExpr.Args[0].(*dst.CallExpr); ok {
									if ident, ok := argCall.Fun.(*dst.Ident); ok {
										if ident.Name == "Middleware" && ident.Path == "github.com/newrelic/go-agent/v3/integrations/nrgin" {
											// Gin middleware is already present
											hasGinTransaction = true
										}
									}
								}
							}
						}
					}
				}
			}
			return true
		}, nil)

		// If we found existing Gin instrumentation, mark the function as already traced
		if hasGinTransaction {
			manager.createFunctionDeclaration(decl)
			manager.updateFunctionDeclaration(decl)
		}
	}
}
