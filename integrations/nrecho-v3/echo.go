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
	echoImportPath    = "github.com/labstack/echo"
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

// DefineTxnFromEchoCtx injects a line of code that extracts a transaction from the echo context into the function body
func DefineTxnFromEchoCtx(body *dst.BlockStmt, txnVariable string, ctxName string) {
	EchoMiddleware.DefineTxnFromCtx(body, txnVariable, ctxName)
}

// Stateful Tracing Functions
// ////////////////////////////////////////////

// InstrumentEchoMiddleware injects nrecho middleware after an echo router creation statement.
func InstrumentEchoMiddleware(manager *parser.InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool {
	routerName := EchoMiddlewareCall(stmt)
	if routerName == "" {
		return false
	}

	if EchoMiddleware.HasExistingMiddleware(c) {
		comment.Debug(manager.GetDecoratorPackage(), stmt, fmt.Sprintf("Skipping echo middleware for router %s: already instrumented", routerName))
		return false
	}

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

		if EchoMiddleware.HasExistingTransaction(v) {
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
