package nrgin

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
	"github.com/newrelic/go-easy-instrumentation/parser"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

const (
	ginImportPath    = "github.com/gin-gonic/gin"
	ginContextObject = "*" + ginImportPath + ".Context"
)

// ginMiddlewareCall returns the variable name of the gin router so that new relic middleware can be appended
func GinMiddlewareCall(stmt dst.Stmt) string {
	v, ok := stmt.(*dst.AssignStmt)
	if !ok || len(v.Rhs) != 1 {
		return ""
	}
	if call, ok := v.Rhs[0].(*dst.CallExpr); ok {
		if ident, ok := call.Fun.(*dst.Ident); ok {
			if (ident.Name == "Default" || ident.Name == "New") && ident.Path == ginImportPath {
				if v.Lhs != nil {
					return v.Lhs[0].(*dst.Ident).Name
				}
			}
		}
	}

	return ""
}

// getGinContextFromHandler checks the type of a function or function literal declaration to determine if
// this is a Gin handler. returns the context variable of the gin handler
func GetGinContextFromHandler(nodeType *dst.FuncType, pkg *decorator.Package) string {
	// gin functions should only have 1 parameter
	if len(nodeType.Params.List) != 1 {
		return ""
	}

	// gin function parameters should only have one named parameter element
	arg := nodeType.Params.List[0]
	if len(arg.Names) != 1 {
		return ""
	}
	argType := util.TypeOf(arg.Type, pkg)
	if argType == nil {
		return ""
	}

	if argType.String() == ginContextObject {
		return arg.Names[0].Name
	}

	return ""
}

// hasExistingGinTransaction checks if a function already has nrgin.Transaction calls
func hasExistingGinTransaction(funcDecl *dst.FuncDecl) bool {
	if funcDecl == nil || funcDecl.Body == nil {
		return false
	}

	hasTransaction := false
	dstutil.Apply(funcDecl.Body, func(c *dstutil.Cursor) bool {
		node := c.Node()
		if stmt, ok := node.(*dst.AssignStmt); ok {
			for _, rhs := range stmt.Rhs {
				if callExpr, ok := rhs.(*dst.CallExpr); ok {
					if ident, ok := callExpr.Fun.(*dst.Ident); ok {
						if ident.Name == "Transaction" && ident.Path == NrginImportPath {
							hasTransaction = true
							return false
						}
					}
				}
			}
		}
		return true
	}, nil)

	return hasTransaction
}

// hasExistingGinMiddleware checks if nrgin.Middleware is already present after the current router assignment
func hasExistingGinMiddleware(c *dstutil.Cursor) bool {
	// Check all remaining statements to see if middleware is already added
	parent := c.Parent()
	if blockStmt, ok := parent.(*dst.BlockStmt); ok {
		// Find current statement index
		currentIndex := -1
		for i, stmt := range blockStmt.List {
			if stmt == c.Node() {
				currentIndex = i
				break
			}
		}

		// Check all remaining statements for existing middleware
		if currentIndex >= 0 {
			for i := currentIndex + 1; i < len(blockStmt.List); i++ {
				if exprStmt, ok := blockStmt.List[i].(*dst.ExprStmt); ok {
					if callExpr, ok := exprStmt.X.(*dst.CallExpr); ok {
						if selExpr, ok := callExpr.Fun.(*dst.SelectorExpr); ok {
							// Check for router.Use(nrgin.Middleware(...))
							if selExpr.Sel.Name == "Use" && len(callExpr.Args) > 0 {
								if argCall, ok := callExpr.Args[0].(*dst.CallExpr); ok {
									if ident, ok := argCall.Fun.(*dst.Ident); ok {
										if ident.Name == "Middleware" && ident.Path == NrginImportPath {
											return true
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return false
}

// defineTxnFromGinCtx injects a line of code that extracts a transaction from the gin context into the function body
func DefineTxnFromGinCtx(body *dst.BlockStmt, txnVariable string, ctxName string) {
	stmts := make([]dst.Stmt, len(body.List)+1)
	stmts[0] = TxnFromGinContext(txnVariable, ctxName)
	for i, stmt := range body.List {
		stmts[i+1] = stmt
	}
	body.List = stmts
}

// Stateful Tracing Functions
// ////////////////////////////////////////////

// WrapHandleFunction is a function that wraps net/http.HandeFunc() declarations inside of functions
// that are being traced by a transaction.
func InstrumentGinMiddleware(manager *parser.InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool {
	// Check if any return true for ginMiddlewareCall
	routerName := GinMiddlewareCall(stmt)
	if routerName == "" {
		return false
	}

	// Check if middleware is already present by looking at the next statement
	if hasExistingGinMiddleware(c) {
		comment.Debug(manager.GetDecoratorPackage(), stmt, fmt.Sprintf("Skipping gin middleware for router %s: already instrumented", routerName))
		return false
	}

	// Append at the current stmt location
	middleware, goGet := NrGinMiddleware(routerName, tracing.AgentVariable())
	comment.Debug(manager.GetDecoratorPackage(), stmt, fmt.Sprintf("Injecting nrgin middleware for router: %s", routerName))
	c.InsertAfter(middleware)
	manager.AddImport(goGet)
	return true
}

// Stateless Tracing Functions
// ////////////////////////////////////////////

// InstrumentGinFunction verifies gin function calls and initiates tracing.
// If tracing was added, then defineTxnFromGinCtx is called to inject the transaction
// into the function body via the gin context
func InstrumentGinFunction(manager *parser.InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	switch v := currentNode.(type) {
	case *dst.FuncDecl:
		ctxName := GetGinContextFromHandler(v.Type, manager.GetDecoratorPackage())
		if ctxName == "" {
			return
		}

		// Check if nrgin.Transaction is already present in the function body
		if hasExistingGinTransaction(v) {
			comment.Debug(manager.GetDecoratorPackage(), v, fmt.Sprintf("Skipping gin handler %s: already has nrgin.Transaction", v.Name.Name))
			return
		}

		comment.Debug(manager.GetDecoratorPackage(), v, fmt.Sprintf("Instrumenting gin handler: %s", v.Name.Name))
		funcDecl := currentNode.(*dst.FuncDecl)
		txnName := codegen.DefaultTransactionVariable
		_, ok := parser.TraceFunction(manager, funcDecl, tracestate.FunctionBody(txnName))
		if ok {
			DefineTxnFromGinCtx(funcDecl.Body, txnName, ctxName)
		}

	case *dst.FuncLit:
		ctxName := GetGinContextFromHandler(v.Type, manager.GetDecoratorPackage())
		if ctxName == "" {
			return
		}

		comment.Debug(manager.GetDecoratorPackage(), v, "Instrumenting gin handler function literal")
		funcLit := currentNode.(*dst.FuncLit)
		txnName := codegen.DefaultTransactionVariable
		tc := tracestate.FunctionBody(codegen.DefaultTransactionVariable).FuncLiteralDeclaration(manager.GetDecoratorPackage(), funcLit)
		tc.CreateSegment(funcLit)
		DefineTxnFromGinCtx(funcLit.Body, txnName, ctxName)
		comment.Warn(manager.GetDecoratorPackage(), c.Parent(), c.Node(), "function literal segments will be named \"function literal\" by default", "declare a function instead to improve segment name generation")
	}
}
