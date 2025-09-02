package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

const (
	// Methods that can be instrumented
	httpHandleFunc = "HandleFunc"
	httpMuxHandle  = "Handle"
	httpDo         = "Do"

	// methods cannot be instrumented
	httpGet      = "Get"
	httpPost     = "Post"
	httpHead     = "Head"
	httpPostForm = "PostForm"

	// default net/http client variable
	httpDefaultClientVariable = "DefaultClient"
)

// all valid generic HTTP method types, or net/http's HandleFunc
var httpMethodTypes = map[string]struct{}{
	"GET":     {},
	"POST":    {},
	"PUT":     {},
	"DELETE":  {},
	"HEAD":    {},
	"TRACE":   {},
	"CONNECT": {},
	"PATCH":   {},
}

// GetNetHttpClientVariableName looks for an http client in the call expression n. If it finds one, the name
// of the variable containing the client will be returned as a string.
func getNetHttpClientVariableName(n *dst.CallExpr, pkg *decorator.Package) string {
	if n == nil {
		return ""
	}

	Sel, ok := n.Fun.(*dst.SelectorExpr)
	if ok {
		switch v := Sel.X.(type) {
		case *dst.SelectorExpr:
			path := util.PackagePath(v.Sel, pkg)
			if path == codegen.HttpImportPath {
				return v.Sel.Name
			}
		case *dst.Ident:
			path := util.PackagePath(v, pkg)
			if path == codegen.HttpImportPath {
				return v.Name
			}
		}
	}
	return ""
}

// GetNetHttpMethod gets an http method if one is invoked in the call expression n, and returns the name of it as a string
func getNetHttpMethod(n *dst.CallExpr, pkg *decorator.Package) string {
	if n == nil {
		return ""
	}

	switch v := n.Fun.(type) {
	case *dst.SelectorExpr:
		path := util.PackagePath(v.Sel, pkg)
		if path == codegen.HttpImportPath {
			return v.Sel.Name
		}
	case *dst.Ident:
		path := util.PackagePath(v, pkg)
		if path == codegen.HttpImportPath {
			return v.Name
		}
	}

	return ""
}

// txnFromCtx injects a line of code that extracts a transaction from the context into the body of a function
func defineTxnFromCtx(fn *dst.FuncDecl, txnVariable string) {
	stmts := make([]dst.Stmt, len(fn.Body.List)+1)
	reqArgName := getHTTPRequestArgNameFnDecl(fn)
	stmts[0] = codegen.TxnFromContext(txnVariable, codegen.HttpRequestContext(reqArgName))
	for i, stmt := range fn.Body.List {
		stmts[i+1] = stmt
	}
	fn.Body.List = stmts
}

func isHttpHandler(decl *dst.FuncDecl, pkg *decorator.Package) bool {
	if pkg == nil {
		return false
	}

	params := decl.Type.Params.List
	if len(params) == 2 {
		var rw, req bool
		for _, param := range params {
			ident, ok := param.Type.(*dst.Ident)
			star, okStar := param.Type.(*dst.StarExpr)
			if ok {
				astNode := pkg.Decorator.Ast.Nodes[ident]
				astIdent, ok := astNode.(*ast.SelectorExpr)
				if ok && pkg.TypesInfo != nil {
					paramType := pkg.TypesInfo.Types[astIdent]
					t := paramType.Type.String()
					if t == "net/http.ResponseWriter" {
						rw = true
					}
				}
			} else if okStar {
				astNode := pkg.Decorator.Ast.Nodes[star]
				astStar, ok := astNode.(*ast.StarExpr)
				if ok && pkg.TypesInfo != nil {
					paramType := pkg.TypesInfo.Types[astStar]
					t := paramType.Type.String()
					if t == "*net/http.Request" {
						req = true
					}
				}
			}
		}
		return rw && req
	}
	return false
}

// more unit test friendly helper function
func isNetHttpClientDefinition(stmt *dst.AssignStmt) bool {
	if len(stmt.Rhs) == 1 && len(stmt.Lhs) == 1 && stmt.Tok == token.DEFINE {
		unary, ok := stmt.Rhs[0].(*dst.UnaryExpr)
		if ok && unary.Op == token.AND {
			lit, ok := unary.X.(*dst.CompositeLit)
			if ok {
				ident, ok := lit.Type.(*dst.Ident)
				if ok && ident.Name == "Client" && ident.Path == codegen.HttpImportPath {
					return true
				}
			}
		}
	}
	return false
}

// StatelessTracingFunctions
//////////////////////////////////////////////

// Recognize if a function is a handler func based on its contents, and inject instrumentation.
// This function discovers entrypoints to tracing for a given transaction and should trace all the way
// down the call chain of the function it is invoked on.
func InstrumentHandleFunction(manager *InstrumentationManager, c *dstutil.Cursor) {
	n := c.Node()
	fn, isFn := n.(*dst.FuncDecl)
	if isFn && isHttpHandler(fn, manager.getDecoratorPackage()) {
		txnName := codegen.DefaultTransactionVariable
		newFn, ok := TraceFunction(manager, fn, tracestate.FunctionBody(txnName))
		if ok {
			defineTxnFromCtx(newFn.(*dst.FuncDecl), txnName) // pass the transaction
		}
	}
}

// InstrumentHttpClient automatically injects a newrelic roundtripper into any newly created http client
// looks for the following pattern: client := &http.Client{}
func InstrumentHttpClient(manager *InstrumentationManager, c *dstutil.Cursor) {
	n := c.Node()
	stmt, ok := n.(*dst.AssignStmt)
	if ok && isNetHttpClientDefinition(stmt) && c.Index() >= 0 && n.Decorations() != nil {
		c.InsertAfter(codegen.RoundTripper(stmt.Lhs[0], n.Decorations().After)) // add roundtripper to transports
		stmt.Decs.After = dst.None
		manager.addImport(codegen.NewRelicAgentImportPath)
	}
}

func cannotTraceOutboundHttp(method string, decs *dst.NodeDecs) []string {
	comment := []string{
		fmt.Sprintf("// the \"http.%s()\" net/http method can not be instrumented and its outbound traffic can not be traced", method),
		"// please see these examples of code patterns for external http calls that can be instrumented:",
		"// https://docs.newrelic.com/docs/apm/agents/go-agent/configuration/distributed-tracing-go-agent/#make-http-requests",
	}

	if decs != nil && len(decs.Start.All()) > 0 {
		comment = append(comment, "//")
	}

	return comment
}

// isNetHttpMethodCannotInstrument is a function that discovers methods of net/http that can not be instrumented by new relic
// and returns the name of the method and whether it can be instrumented or not.
func isNetHttpMethodCannotInstrument(node dst.Node) (string, bool) {
	var cannotInstrument bool
	var returnFuncName string

	switch node.(type) {
	case *dst.AssignStmt, *dst.ExprStmt:
		dst.Inspect(node, func(n dst.Node) bool {
			_, block := n.(*dst.BlockStmt)
			if block {
				return false
			}

			c, ok := n.(*dst.CallExpr)
			if ok {
				ident, ok := c.Fun.(*dst.Ident)
				if ok && ident.Path == codegen.HttpImportPath {
					switch ident.Name {
					case httpGet, httpPost, httpPostForm, httpHead:
						returnFuncName = ident.Name
						cannotInstrument = true
						return false
					}
				}
			}
			return true
		})
	}
	return returnFuncName, cannotInstrument
}

// CannotInstrumentHttpMethod is a function that discovers methods of net/http. If that function can not be penetrated by
// instrumentation, it leaves a comment header warning the customer. This function needs no tracing context to work.
func CannotInstrumentHttpMethod(manager *InstrumentationManager, c *dstutil.Cursor) {
	n := c.Node()
	funcName, ok := isNetHttpMethodCannotInstrument(n)
	if ok {
		if decl := n.Decorations(); decl != nil {
			decl.Start.Prepend(cannotTraceOutboundHttp(funcName, n.Decorations())...)
		}
	}
}

// StateFull TracingFunctions
//////////////////////////////////////////////

// getHttpResponseVariable returns the expression that contains an object of `*net/http.Response` type
func getHttpResponseVariable(manager *InstrumentationManager, stmt dst.Stmt) dst.Expr {
	var expression dst.Expr
	pkg := manager.getDecoratorPackage()
	dst.Inspect(stmt, func(n dst.Node) bool {
		switch v := n.(type) {
		case *dst.BlockStmt:
			return false
		case *dst.AssignStmt:
			for _, expr := range v.Lhs {
				astExpr := pkg.Decorator.Ast.Nodes[expr]
				if astExpr == nil {
					return true
				}

				t := pkg.TypesInfo.TypeOf(astExpr.(ast.Expr))
				if t != nil && t.String() == "*net/http.Response" {
					expression = expr
					return false
				}
			}
		}
		return true
	})
	return expression
}

// ExternalHttpCall finds and instruments external net/http calls to the method http.Do.
// It returns true if a modification was made
func ExternalHttpCall(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool {
	if c.Index() < 0 {
		return false
	}
	pkg := manager.getDecoratorPackage()
	var call *dst.CallExpr
	dst.Inspect(stmt, func(n dst.Node) bool {
		switch v := n.(type) {
		case *dst.BlockStmt:
			return false
		case *dst.CallExpr:
			if getNetHttpMethod(v, pkg) == httpDo {
				call = v
				return false
			}
		}
		return true
	})
	if call != nil && c.Index() >= 0 {
		clientVar := getNetHttpClientVariableName(call, pkg)
		requestObject := call.Args[0]
		if clientVar == httpDefaultClientVariable {
			// create external segment to wrap calls made with default client
			segmentName := "externalSegment"
			c.InsertBefore(codegen.StartExternalSegment(requestObject, tracing.TransactionVariable(), segmentName, stmt.Decorations()))
			c.InsertAfter(codegen.EndExternalSegment(segmentName, stmt.Decorations()))
			responseVar := getHttpResponseVariable(manager, stmt)
			manager.addImport(codegen.NewRelicAgentImportPath)
			if responseVar != nil {
				c.InsertAfter(codegen.CaptureHttpResponse(segmentName, responseVar))
			}
			return true
		} else {
			c.InsertBefore(codegen.WrapRequestContext(requestObject, tracing.TransactionVariable(), stmt.Decorations()))
			manager.addImport(codegen.NewRelicAgentImportPath)
			return true
		}
	}
	return false
}

// WrapHandleFunction is a function that wraps net/http.HandeFunc() declarations inside of functions
// that are being traced by a transaction.
func WrapNestedHandleFunction(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool {
	wasModified := false
	pkg := manager.getDecoratorPackage()
	dst.Inspect(stmt, func(n dst.Node) bool {
		switch v := n.(type) {
		case *dst.BlockStmt:
			return false
		case *dst.CallExpr:
			callExpr := v
			funcName := getNetHttpMethod(callExpr, pkg)
			switch funcName {
			case httpHandleFunc:
				if len(callExpr.Args) == 2 {
					// Instrument handle funcs
					codegen.WrapHttpHandleFunc(tracing.AgentVariable(), callExpr)

					wasModified = true
					manager.addImport(codegen.NewRelicAgentImportPath)
					return false
				}
			case httpMuxHandle:
				if len(callExpr.Args) == 2 {
					// Instrument handle funcs
					codegen.WrapHttpHandle(tracing.AgentVariable(), callExpr)

					wasModified = true
					manager.addImport(codegen.NewRelicAgentImportPath)
					return false
				}
			}
		}
		return true
	})
	return wasModified
}

// verify the existence of the HandleFunc call
// http.HandleFunc("/route", func(w, r){...})
// _____^^^^^^^^^^
func getHTTPHandleFunc(node dst.Node) *dst.CallExpr {
	switch v := node.(type) {
	case *dst.ExprStmt:
		call, ok := v.X.(*dst.CallExpr)
		if !ok {
			return nil
		}

		ident, ok := call.Fun.(*dst.Ident)
		if !ok {
			return nil
		}

		if ident.Path != codegen.HttpImportPath {
			return nil
		}

		if ident.Name != "HandleFunc" {
			return nil
		}

		return call
	}

	return nil
}

// extract the HTTP method type from the route handler declaration
// r.Get("/routename", func(w,r){...})
// __^^^
func getHTTPMethodType(node dst.Node) (string, *dst.CallExpr) {
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

		method := strings.ToUpper(selExpr.Sel.Name)
		_, ok = httpMethodTypes[method]
		if !ok {
			return "", nil
		}

		return method, call
	}
	return "", nil
}

// extract the route name from the route handler declaration
// r.Get("/routename", func(w,r){...})
// _______^^^^^^^^^^
func getHTTPHandlerRouteName(callExpr *dst.CallExpr) (string, *dst.FuncLit) {
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

// extract the request arg name from the declared route handler
// func fnDecl(w http.ResponseWriter, r *http.Request) {...}
// ___________________________________^
func getHTTPRequestArgNameFnDecl(fnDecl *dst.FuncDecl) string {
	if fnDecl == nil {
		return ""
	}

	if fnDecl.Type == nil || fnDecl.Type.Params == nil || fnDecl.Type.Params.List == nil {
		return ""
	}

	if len(fnDecl.Type.Params.List) != 2 {
		return ""
	}

	reqArg := fnDecl.Type.Params.List[1]

	if len(reqArg.Names) != 1 {
		return ""
	}

	return reqArg.Names[0].Name
}

// extract the request arg name from the anonymous route handler
// r.Get("/routename", func(w http.ResponseWriter, r *http.Request){...})
// ________________________________________________^
func getHTTPRequestArgNameFnLit(fnLit *dst.FuncLit) string {
	if fnLit == nil {
		return ""
	}

	if fnLit.Type == nil || fnLit.Type.Params == nil || fnLit.Type.Params.List == nil {
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

// InstrumentRouteHandlerFuncLit adds instrumentation for router function literal handlers
// For a Route Handler function literal to be considered, it must satisfy the following constraints:
// 1. A valid HTTP method type (GET, POST, DELETE, etc)
// 2. A valid route name (/index, /route/literal, etc) defined as the first argument to the route method
// 3. A function literal as the second argument to the route method
// 4. An http.ResponseWriter and http.Request argument to the function literal
//
// If all constraints above are satisfied, an NR txn object is retreived via the request context, and
// injected alongside a defer segment start with the segment name comprising of the HTTP method +":routename"
func InstrumentRouteHandlerFuncLit(manager *InstrumentationManager, c *dstutil.Cursor) {
	methodName, callExpr := getHTTPMethodType(c.Node())

	if methodName == "" || callExpr == nil {
		return
	}

	routeName, fnLit := getHTTPHandlerRouteName(callExpr)
	if routeName == "" || fnLit == nil {
		return
	}

	reqArgName := getHTTPRequestArgNameFnLit(fnLit)
	if reqArgName == "" {
		return
	}

	txn := codegen.TxnFromContext(codegen.DefaultTransactionVariable, codegen.HttpRequestContext(reqArgName))
	if txn == nil {
		return
	}

	segmentName := methodName + ":" + routeName
	codegen.PrependStatementToFunctionLit(fnLit, codegen.DeferSegment(segmentName, dst.NewIdent(codegen.DefaultTransactionVariable)))
	codegen.PrependStatementToFunctionLit(fnLit, txn)
	manager.addImport(codegen.NewRelicAgentImportPath)
}

// InstrumentHTTPHandleFuncLit adds instrumentation for http HandleFunc function literals
// For an http Handler function literal to be considered, it must satisfy the following constraints:
// 1. A call to HandleFunc
// 2. A valid route name (/index, /route/literal, etc) defined as the first argument to the route method
// 3. A function literal as the second argument to the route method
// 4. An http.ResponseWriter and http.Request argument to the function literal
//
// If all constraints above are satisfied, an NR txn object is retreived via the request context, and
// injected alongside a defer segment start with the segment name comprising of the HTTP method +":routename"
func InstrumentHTTPHandleFuncLit(manager *InstrumentationManager, c *dstutil.Cursor) {
	callExpr := getHTTPHandleFunc(c.Node())

	if callExpr == nil {
		return
	}

	routeName, fnLit := getHTTPHandlerRouteName(callExpr)
	routeName, err := strconv.Unquote(routeName)
	if routeName == "" || fnLit == nil || err != nil {
		return
	}

	reqArgName := getHTTPRequestArgNameFnLit(fnLit)
	if reqArgName == "" {
		return
	}

	txn := codegen.TxnFromContext(codegen.DefaultTransactionVariable, codegen.HttpRequestContext(reqArgName))
	if txn == nil {
		return
	}

	segmentName := "http.HandleFunc" + ":" + routeName
	codegen.PrependStatementToFunctionLit(fnLit, codegen.DeferSegment(segmentName, dst.NewIdent(codegen.DefaultTransactionVariable)))
	codegen.PrependStatementToFunctionLit(fnLit, txn)
	manager.addImport(codegen.NewRelicAgentImportPath)
}
