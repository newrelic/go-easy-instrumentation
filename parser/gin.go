package parser

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

const (
	ginImportPath    = "github.com/gin-gonic/gin"
	ginContextObject = "*" + ginImportPath + ".Context"
)

// ginMiddlewareCall returns the variable name of the gin router so that new relic middleware can be appended
func ginMiddlewareCall(stmt dst.Stmt) string {
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
func getGinContextFromHandler(nodeType *dst.FuncType, pkg *decorator.Package) string {
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

// defineTxnFromGinCtx injects a line of code that extracts a transaction from the gin context into the function body
func defineTxnFromGinCtx(body *dst.BlockStmt, txnVariable string, ctxName string) {
	stmts := make([]dst.Stmt, len(body.List)+1)
	stmts[0] = codegen.TxnFromGinContext(txnVariable, ctxName)
	for i, stmt := range body.List {
		stmts[i+1] = stmt
	}
	body.List = stmts
}

// Stateful Tracing Functions
// ////////////////////////////////////////////

// WrapHandleFunction is a function that wraps net/http.HandeFunc() declarations inside of functions
// that are being traced by a transaction.
func InstrumentGinMiddleware(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool {
	// Check if any return true for ginMiddlewareCall
	routerName := ginMiddlewareCall(stmt)
	if routerName == "" {
		return false
	}
	// Append at the current stmt location
	middleware, goGet := codegen.NrGinMiddleware(routerName, tracing.AgentVariable())
	comment.Debug(manager.getDecoratorPackage(), stmt, fmt.Sprintf("Injecting nrgin middleware for router: %s", routerName))
	c.InsertAfter(middleware)
	manager.addImport(goGet)
	return true
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
		ctxName := getGinContextFromHandler(v.Type, manager.getDecoratorPackage())
		if ctxName == "" {
			return
		}

		comment.Debug(manager.getDecoratorPackage(), v, fmt.Sprintf("Instrumenting gin handler: %s", v.Name.Name))
		funcDecl := currentNode.(*dst.FuncDecl)
		txnName := codegen.DefaultTransactionVariable
		_, ok := TraceFunction(manager, funcDecl, tracestate.FunctionBody(txnName))
		if ok {
			defineTxnFromGinCtx(funcDecl.Body, txnName, ctxName)
		}

	case *dst.FuncLit:
		ctxName := getGinContextFromHandler(v.Type, manager.getDecoratorPackage())
		if ctxName == "" {
			return
		}

		comment.Debug(manager.getDecoratorPackage(), v, "Instrumenting gin handler function literal")
		funcLit := currentNode.(*dst.FuncLit)
		txnName := codegen.DefaultTransactionVariable
		tc := tracestate.FunctionBody(codegen.DefaultTransactionVariable).FuncLiteralDeclaration(manager.getDecoratorPackage(), funcLit)
		tc.CreateSegment(funcLit)
		defineTxnFromGinCtx(funcLit.Body, txnName, ctxName)
		comment.Warn(manager.getDecoratorPackage(), c.Parent(), c.Node(), "function literal segments will be named \"function literal\" by default", "declare a function instead to improve segment name generation")
	}
}
