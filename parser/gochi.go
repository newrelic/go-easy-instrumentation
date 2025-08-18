package parser

import (
	"go/token"
	"strconv"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

const (
	gochiImportPath = "github.com/go-chi/chi/v5"
)

// Return the variable name of the Chi router object.
// Ex:
//
//	router := chi.NewRouter()
//
// would return "router"
func getChiRouterName(stmt dst.Stmt) string {
	// Verify we're dealing with an assignment operation
	v, ok := stmt.(*dst.AssignStmt)
	if !ok || len(v.Rhs) != 1 {
		return ""
	}

	if v.Lhs == nil {
		return ""
	}

	// Verify the Rhs of the assignment is a Call Expression
	call, ok := v.Rhs[0].(*dst.CallExpr)
	if !ok {
		return ""
	}

	// Verify the name and path of the function being called
	ident, ok := call.Fun.(*dst.Ident)
	if !ok {
		return ""
	}

	// Reject calls that are not to the `NewRouter` Fn. Verify Chi relationship with the import path.
	if ident.Name != "NewRouter" || ident.Path != gochiImportPath {
		return ""
	}

	return v.Lhs[0].(*dst.Ident).Name
}

// Extract the HTTP method type and CallExpr node from the current cursor node
func getChiHTTPMethod(node dst.Node) (string, *dst.CallExpr) {
	switch v := node.(type) {
	case *dst.ExprStmt:
		call, ok := v.X.(*dst.CallExpr)
		if !ok {
			return "", nil
		}

		selExpr, ok := call.Fun.(*dst.SelectorExpr)
		if !ok {
			return "", nil
		}

		method := selExpr.Sel.Name
		switch strings.ToUpper(method) {
		case "GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "TRACE", "CONNECT", "PATCH":
			return strings.ToUpper(method), call
		default:
			return "", nil
		}
	}
	return "", nil
}

// Get the name of the route being registered to the handler for naming purposes
func getChiHTTPHandlerRouteName(callExpr *dst.CallExpr) (string, *dst.FuncLit) {
	if callExpr == nil {
		return "", nil
	}

	if len(callExpr.Args) != 2 {
		return "", nil
	}

	routeName, ok := callExpr.Args[0].(*dst.BasicLit)
	if !ok || routeName.Kind != token.STRING {
		return "", nil
	}

	fnLit, ok := callExpr.Args[1].(*dst.FuncLit)
	if !ok {
		return "", nil
	}

	return routeName.Value, fnLit
}

// Get the name of the *http.Request arg in the function literal http handler
func getChiHTTPRequestArgName(fnLit *dst.FuncLit) string {
	if fnLit == nil {
		return ""
	}

	if len(fnLit.Type.Params.List) != 2 {
		return ""
	}

	reqArg := fnLit.Type.Params.List[1]

	if len(reqArg.Names) != 1 {
		return ""
	}

	return reqArg.Names[0].Name
}

// InstrumentChiMiddleware detects whether a Chi Router has been initialized
// and adds New Relic Go Agent Middleware via the router.Use() method to
// instrument the routes registered to the router.
func InstrumentChiMiddleware(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool {
	routerName := getChiRouterName(stmt)
	if routerName == "" {
		return false
	}

	// Append at the current stmt location
	middleware, goGet := codegen.NrChiMiddleware(routerName, tracing.AgentVariable())
	c.InsertAfter(middleware)
	manager.addImport(goGet)
	return true
}

// InstrumentChiRouterLiteral detects if a Chi Router route uses a function
// literal and adds Txn/Segment tracing logic directly to the function literal
// block.
func InstrumentChiRouterLiteral(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool {
	methodName, callExpr := getChiHTTPMethod(c.Node())
	if methodName == "" || callExpr == nil {
		return false
	}

	routeName, fnLit := getChiHTTPHandlerRouteName(callExpr)
	routeName, err := strconv.Unquote(routeName)
	if routeName == "" || fnLit == nil || err != nil {
		return false
	}

	reqArgName := getChiHTTPRequestArgName(fnLit)
	if reqArgName == "" {
		return false
	}

	txn := codegen.TxnFromContext(codegen.DefaultTransactionVariable, codegen.HttpRequestContext(reqArgName))
	if txn == nil {
		return false
	}

	segmentName := methodName + ":" + routeName

	codegen.PrependStatementToFunctionLit(fnLit, codegen.DeferSegment(segmentName, tracing.TransactionVariable()))
	codegen.PrependStatementToFunctionLit(fnLit, txn)

	return true
}
