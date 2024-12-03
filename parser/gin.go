package parser

import (
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

const (
	NrginImportPath                = "github.com/newrelic/go-agent/v3/integrations/nrgin"
	ginImportPath                  = "github.com/gin-gonic/gin"
	ginContextObject               = "*" + ginImportPath + ".Context"
	NewRelicAgentImportPath string = "github.com/newrelic/go-agent/v3/newrelic"
)

func ginMiddlewareCall(node dst.Node) (*dst.CallExpr, bool, string) {
	switch v := node.(type) {
	case *dst.AssignStmt:
		if len(v.Rhs) == 1 {
			if call, ok := v.Rhs[0].(*dst.CallExpr); ok {
				if ident, ok := call.Fun.(*dst.Ident); ok {
					if ident.Name == "Default" || ident.Name == "New" && ident.Path == ginImportPath {
						if v.Lhs != nil {
							return call, true, v.Lhs[0].(*dst.Ident).Name
						}
					}
				}
			}
		}
	}
	return nil, false, ""
}

// isGinHandler checks the type of a function or function literal declaration to determine if
// this is a Gin handler
func isGinHandler(nodeType *dst.FuncType, pkg *decorator.Package) (string, bool) {
	// gin functions should only have 1 parameter
	if len(nodeType.Params.List) != 1 {
		return "", false
	}

	// gin function parameters should only have one named parameter element
	arg := nodeType.Params.List[0]
	if len(arg.Names) != 1 {
		return "", false
	}

	argType := util.TypeOf(arg.Type, pkg)
	if argType == nil {
		return "", false
	}

	if argType.String() == ginContextObject {
		return arg.Names[0].Name, true
	}

	return "", false
}

func checkForGinContextFuncDecl(decl *dst.FuncDecl, pkg *decorator.Package) (string, bool) {
	if decl.Name == nil {
		return "", false
	}

	return isGinHandler(decl.Type, pkg)
}

func checkForGinContextFuncLit(funcLit *dst.FuncLit, pkg *decorator.Package) (string, bool) {
	return isGinHandler(funcLit.Type, pkg)
}

// defineTxnFromGinCtx injects a line of code that extracts a transaction from the gin context into the function body
func defineTxnFromGinCtx(fn *dst.FuncDecl, txnVariable string, ctxName string) {
	stmts := make([]dst.Stmt, len(fn.Body.List)+1)
	stmts[0] = codegen.TxnFromGinContext(txnVariable, ctxName)
	for i, stmt := range fn.Body.List {
		stmts[i+1] = stmt
	}
	fn.Body.List = stmts
}
func defineTxnFromGinCtxLit(manager *InstrumentationManager, fn *dst.FuncLit, txnVariable string, ctxName string) {
	stmts := make([]dst.Stmt, len(fn.Body.List)+1)
	// If the function is a gin anonymous route, append the comment after the transaction is defined
	stmts[0] = codegen.TxnFromGinContext(txnVariable, ctxName)
	for i, stmt := range fn.Body.List {
		stmts[i+1] = stmt
	}
	if !manager.anonymousFunctionWarning {
		manager.anonymousFunctionWarning = true
		comment.Warn(manager.getDecoratorPackage(), stmts[0], "Since the handler function name is used as the transaction name, anonymous functions do not get usefully named.", "We encourage transforming anonymous functions into named functions")
	}
	fn.Body.List = stmts
}

// Stateful Tracing Functions
// ////////////////////////////////////////////

// WrapHandleFunction is a function that wraps net/http.HandeFunc() declarations inside of functions
// that are being traced by a transaction.
func InstrumentGinMiddleware(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool {
	// Check if any return true for ginMiddlewareCall
	if _, ok, routerName := ginMiddlewareCall(stmt); ok {
		// Append at the current stmt location
		c.InsertAfter(codegen.NrGinMiddleware(routerName, tracing.AgentVariable()))
		return true
	}
	return false
}

// Stateless Tracing Functions
// ////////////////////////////////////////////

// InstrumentGinFunction verifies gin function calls and initiates tracing.
// If tracing was added, then defineTxnFromGinCtx is called to inject the transaction
// into the function body via the gin context
func InstrumentGinFunction(manager *InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	switch v := currentNode.(type) {
	case *dst.FuncDecl:
		if ctxName, ok := checkForGinContextFuncDecl(v, manager.getDecoratorPackage()); ok {
			funcDecl := currentNode.(*dst.FuncDecl)
			txnName := codegen.DefaultTransactionVariable
			_, ok := TraceFunction(manager, funcDecl, tracestate.FunctionBody(txnName))
			if ok {
				defineTxnFromGinCtx(funcDecl, txnName, ctxName)
			}
		}
	case *dst.FuncLit:
		if ctxName, ok := checkForGinContextFuncLit(v, manager.getDecoratorPackage()); ok {
			funcLit := currentNode.(*dst.FuncLit)
			txnName := codegen.DefaultTransactionVariable
			tc := tracestate.FunctionBody(codegen.DefaultTransactionVariable).FuncLiteralDeclaration(manager.getDecoratorPackage(), funcLit)
			tc.CreateSegment(funcLit)
			defineTxnFromGinCtxLit(manager, funcLit, txnName, ctxName)
		}
	}
}
