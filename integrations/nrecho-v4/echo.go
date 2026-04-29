package nrecho

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
	echoImportPath    = "github.com/labstack/echo/v4"
	echoContextObject = echoImportPath + ".Context"
)

// EchoMiddlewareCall returns the variable name of the echo router so that new relic middleware can be appended
func EchoMiddlewareCall(stmt dst.Stmt) string {
	v, ok := stmt.(*dst.AssignStmt)
	if !ok || len(v.Rhs) != 1 {
		return ""
	}
	if call, ok := v.Rhs[0].(*dst.CallExpr); ok {
		if ident, ok := call.Fun.(*dst.Ident); ok {
			if ident.Name == "New" && ident.Path == echoImportPath {
				if v.Lhs != nil {
					return v.Lhs[0].(*dst.Ident).Name
				}
			}
		}
	}

	return ""
}

// GetEchoContextFromHandler checks the type of a function or function literal declaration to determine if
// this is an Echo handler. Returns the context variable of the echo handler.
func GetEchoContextFromHandler(nodeType *dst.FuncType, pkg *decorator.Package) string {
	// echo functions should only have 1 parameter
	if len(nodeType.Params.List) != 1 {
		return ""
	}

	// echo function parameters should only have one named parameter element
	arg := nodeType.Params.List[0]
	if len(arg.Names) != 1 {
		return ""
	}
	argType := util.TypeOf(arg.Type, pkg)
	if argType == nil {
		return ""
	}

	if argType.String() == echoContextObject {
		return arg.Names[0].Name
	}

	return ""
}

// hasExistingEchoTransaction checks if a function already has nrecho.FromContext calls
func hasExistingEchoTransaction(funcDecl *dst.FuncDecl) bool {
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
						if ident.Name == "FromContext" && ident.Path == NrechoImportPath {
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

// hasExistingEchoMiddleware checks if nrecho.Middleware is already present after the current router assignment
func hasExistingEchoMiddleware(c *dstutil.Cursor) bool {
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
							// Check for router.Use(nrecho.Middleware(...))
							if selExpr.Sel.Name == "Use" && len(callExpr.Args) > 0 {
								if argCall, ok := callExpr.Args[0].(*dst.CallExpr); ok {
									if ident, ok := argCall.Fun.(*dst.Ident); ok {
										if ident.Name == "Middleware" && ident.Path == NrechoImportPath {
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

// DefineTxnFromEchoCtx injects a line of code that extracts a transaction from the echo context into the function body
func DefineTxnFromEchoCtx(body *dst.BlockStmt, txnVariable string, ctxName string) {
	stmts := make([]dst.Stmt, len(body.List)+1)
	stmts[0] = TxnFromEchoContext(txnVariable, ctxName)
	for i, stmt := range body.List {
		stmts[i+1] = stmt
	}
	body.List = stmts
}

// Stateful Tracing Functions
// ////////////////////////////////////////////

// InstrumentEchoMiddleware injects nrecho middleware after an echo router creation statement.
func InstrumentEchoMiddleware(manager *parser.InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool {
	routerName := EchoMiddlewareCall(stmt)
	if routerName == "" {
		return false
	}

	// Check if middleware is already present by looking at the next statement
	if hasExistingEchoMiddleware(c) {
		comment.Debug(manager.GetDecoratorPackage(), stmt, fmt.Sprintf("Skipping echo middleware for router %s: already instrumented", routerName))
		return false
	}

	// Append at the current stmt location
	middleware, goGet := NrEchoMiddleware(routerName, tracing.AgentVariable())
	comment.Debug(manager.GetDecoratorPackage(), stmt, fmt.Sprintf("Injecting nrecho middleware for router: %s", routerName))
	c.InsertAfter(middleware)
	manager.AddImport(goGet)
	return true
}

// Stateless Tracing Functions
// ////////////////////////////////////////////

// InstrumentEchoFunction verifies echo function calls and initiates tracing.
// If tracing was added, then DefineTxnFromEchoCtx is called to inject the transaction
// into the function body via the echo context.
func InstrumentEchoFunction(manager *parser.InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	switch v := currentNode.(type) {
	case *dst.FuncDecl:
		ctxName := GetEchoContextFromHandler(v.Type, manager.GetDecoratorPackage())
		if ctxName == "" {
			return
		}

		// Check if nrecho.FromContext is already present in the function body
		if hasExistingEchoTransaction(v) {
			comment.Debug(manager.GetDecoratorPackage(), v, fmt.Sprintf("Skipping echo handler %s: already has nrecho.FromContext", v.Name.Name))
			return
		}

		comment.Debug(manager.GetDecoratorPackage(), v, fmt.Sprintf("Instrumenting echo handler: %s", v.Name.Name))
		funcDecl := currentNode.(*dst.FuncDecl)
		txnName := codegen.DefaultTransactionVariable
		// Don't use TraceFunction for echo handlers - just add segment like we do for function literals
		tc := tracestate.FunctionBody(txnName).FuncLiteralDeclaration(manager.GetDecoratorPackage(), nil)
		if _, ok := tc.CreateSegment(funcDecl); ok {
			DefineTxnFromEchoCtx(funcDecl.Body, txnName, ctxName)
		}

	case *dst.FuncLit:
		ctxName := GetEchoContextFromHandler(v.Type, manager.GetDecoratorPackage())
		if ctxName == "" {
			return
		}

		comment.Debug(manager.GetDecoratorPackage(), v, "Instrumenting echo handler function literal")
		funcLit := currentNode.(*dst.FuncLit)
		txnName := codegen.DefaultTransactionVariable
		tc := tracestate.FunctionBody(codegen.DefaultTransactionVariable).FuncLiteralDeclaration(manager.GetDecoratorPackage(), funcLit)
		tc.CreateSegment(funcLit)
		DefineTxnFromEchoCtx(funcLit.Body, txnName, ctxName)
		comment.Warn(manager.GetDecoratorPackage(), c.Parent(), c.Node(), "function literal segments will be named \"function literal\" by default", "declare a function instead to improve segment name generation")
	}
}
