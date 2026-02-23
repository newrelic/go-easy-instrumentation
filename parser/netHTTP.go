package parser

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
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

// RouterHasMiddleware detects already existing net/http routers and marks them within the scope of the given transaction.
// It returns true if the function name matches within a wrapped HandleFunc, false otherwise.
// TO:DO -- Can this be extended to ALL routing libraries?
func HandlerIsInstrumented(manager *InstrumentationManager, fn *dst.FuncDecl) bool {
	txns := manager.transactionCache.Transactions
	for ident := range txns {
		if ident == fn.Name {
			return true
		}
	}
	return false
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

// extract the request arg name from a declared route handler
// func handler(w http.ResponseWriter, r *http.Request)
// ____________________________________^
func getHTTPRequestArgNameDecl(fn *dst.FuncDecl) (bool, string) {
	if !isHTTPHandlerDecl(fn) {
		return false, ""
	}

	return true, fn.Type.Params.List[1].Names[0].Name
}

// extract the request arg name from a literal route handler
// func(w http.ResponseWriter, r *http.Request)
// ____________________________^
func getHTTPRequestArgNameLit(fn *dst.FuncLit) (bool, string) {
	if !isHTTPHandlerLit(fn) {
		return false, ""
	}

	return true, fn.Type.Params.List[1].Names[0].Name
}

// wrapper for HTTP request arg extraction.
// take in `any` and filter non FuncDecl and non FuncLit, then
// dispatch to appropriate function.
func getHTTPRequestArgName(fn any) (bool, string) {
	if fn == nil {
		return false, ""
	}

	var ok = false
	var name = ""

	switch f := fn.(type) {
	case *dst.FuncDecl:
		ok, name = getHTTPRequestArgNameDecl(f)
	case *dst.FuncLit:
		ok, name = getHTTPRequestArgNameLit(f)
	default:
		return false, ""
	}
	return ok, name
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
	ok, reqArgName := getHTTPRequestArgName(fn)
	if !ok {
		// TODO: consider injecting a comment or creating a log message describing the failure here.
		return
	}
	stmts[0] = codegen.TxnFromContext(txnVariable, codegen.HttpRequestContext(reqArgName))
	for i, stmt := range fn.Body.List {
		stmts[i+1] = stmt
	}
	fn.Body.List = stmts
}

func isHTTPHandler(fn any) bool {
	switch f := fn.(type) {
	case *dst.FuncLit:
		return isHTTPHandlerLit(f)
	case *dst.FuncDecl:
		return isHTTPHandlerDecl(f)
	default:
		return false
	}
}

func isHTTPResponseWriter(respW *dst.Field) bool {
	if len(respW.Names) != 1 {
		return false
	}

	// NOTE: This should be an Ident, not a SelectorExpr, since package.Func() is
	// considered a Qualified Identifier in Go, not a Selector
	// Sources:
	// - https://go.dev/ref/spec#Selectors
	// - https://go.dev/ref/spec#Qualified_identifiers
	identRespW, ok := respW.Type.(*dst.Ident)
	if !ok {
		return false
	}

	if identRespW.Path != codegen.HttpImportPath || identRespW.Name != "ResponseWriter" {
		return false
	}
	return true
}

func isHTTPRequest(req *dst.Field) bool {
	if len(req.Names) != 1 {
		return false
	}

	starExprReq, ok := req.Type.(*dst.StarExpr)
	if !ok {
		return false
	}

	// NOTE: This should be an Ident, not a SelectorExpr, since package.Func() is
	// considered a Qualified Identifier in Go, not a Selector
	// Sources:
	// - https://go.dev/ref/spec#Selectors
	// - https://go.dev/ref/spec#Qualified_identifiers
	identReq, ok := starExprReq.X.(*dst.Ident)
	if !ok {
		return false
	}

	if identReq.Path != codegen.HttpImportPath || identReq.Name != "Request" {
		return false
	}
	return true
}

// determine whether the funcdecl is an http handler by checking for http
// handler argument signatures
// func myHandler(w http.ResponseWriter, r *http.Request)
// _______________^______________________^
func isHTTPHandlerDecl(fn *dst.FuncDecl) bool {
	if fn == nil || fn.Type == nil || fn.Type.Params == nil || fn.Type.Params.List == nil {
		return false
	}

	if len(fn.Type.Params.List) != 2 {
		return false
	}

	respW := fn.Type.Params.List[0]
	req := fn.Type.Params.List[1]

	if !isHTTPResponseWriter(respW) {
		return false
	}

	if !isHTTPRequest(req) {
		return false
	}

	return true
}

// determine whether the funclit is an http handler by checking for http
// handler argument signatures
// func(w http.ResponseWriter, r *http.Request)
// _____^______________________^
func isHTTPHandlerLit(fn *dst.FuncLit) bool {
	if fn == nil || fn.Type == nil || fn.Type.Params == nil || fn.Type.Params.List == nil {
		return false
	}

	if len(fn.Type.Params.List) != 2 {
		return false
	}

	respW := fn.Type.Params.List[0]
	req := fn.Type.Params.List[1]

	if !isHTTPResponseWriter(respW) {
		return false
	}

	if !isHTTPRequest(req) {
		return false
	}

	return true
}

func isTransportInstrumented(stmt dst.Stmt, clientVarName string) bool {
	assignStmt, ok := stmt.(*dst.AssignStmt)
	if !ok {
		return false
	}

	// Check if LHS is client.Transport
	if len(assignStmt.Lhs) != 1 {
		return false
	}

	selExpr, ok := assignStmt.Lhs[0].(*dst.SelectorExpr)
	if !ok || selExpr.Sel.Name != "Transport" {
		return false
	}

	// Check if selector is the client variable
	if ident, ok := selExpr.X.(*dst.Ident); !ok || ident.Name != clientVarName {
		return false
	}

	// Check if RHS is newrelic.NewRoundTripper
	if len(assignStmt.Rhs) != 1 {
		return false
	}

	callExpr, ok := assignStmt.Rhs[0].(*dst.CallExpr)
	if !ok {
		return false
	}
	funIdent, ok := callExpr.Fun.(*dst.Ident)
	if !ok {
		return false
	}

	return funIdent.Name == "NewRoundTripper" && funIdent.Path == codegen.NewRelicAgentImportPath
}

// clientTransportAlreadyInstrumented checks if the client's Transport is already set to newrelic.NewRoundTripper
// In the statements within the block
func clientTransportAlreadyInstrumented(c *dstutil.Cursor, clientVarName string) bool {
	parent := c.Parent()

	var blockStmt *dst.BlockStmt
	for parent != nil {
		if block, ok := parent.(*dst.BlockStmt); ok {
			blockStmt = block
			break
		}
		parent = c.Parent()
	}

	if blockStmt == nil {
		return false
	}
	// Get current statements index
	currIndx := c.Index()
	if currIndx < 0 {
		return false
	}

	// Check the statements for existing Transport Instrumentation
	for i := 0; currIndx+i < len(blockStmt.List); i++ {
		stmt := blockStmt.List[currIndx+i]
		if isTransportInstrumented(stmt, clientVarName) {
			return true
		}
	}
	return false
}

// more unit test friendly helper function
func isNetHttpClientDefinition(stmt *dst.AssignStmt) bool {
	if len(stmt.Rhs) == 1 && len(stmt.Lhs) == 1 && (stmt.Tok == token.DEFINE) {
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
	fn, isFn := n.(*dst.FuncDecl) // TODO: 'isFn' should be renamed to 'ok' to match the paradigm in the rest of the codebase.
	if isFn && isHTTPHandler(fn) && !HandlerIsInstrumented(manager, fn) {

		comment.Debug(manager.getDecoratorPackage(), fn, fmt.Sprintf("Instrumenting HTTP handler: %s", fn.Name.Name))
		txnName := codegen.DefaultTransactionVariable
		newFn, ok := TraceFunction(manager, fn, tracestate.FunctionBody(txnName))
		if ok {
			defineTxnFromCtx(newFn.(*dst.FuncDecl), txnName) // pass the transaction
		}
	}

}

// InstrumentHttpClient automatically injects a newrelic roundtripper into any newly created http client
// looks for the following pattern: client := &http.Client{}
// Additionally, it also checks if the transport is already instrumented to avoid duplicate injection
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
			comment.Debug(manager.getDecoratorPackage(), stmt, "Wrapping default HTTP client call with external segment")
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
			comment.Debug(manager.getDecoratorPackage(), stmt, "Injecting transaction context into HTTP request")
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
					comment.Debug(manager.getDecoratorPackage(), stmt, "Wrapping http.HandleFunc with newrelic.WrapHandleFunc")
					codegen.WrapHttpHandleFunc(tracing.AgentVariable(), callExpr)

					wasModified = true
					manager.addImport(codegen.NewRelicAgentImportPath)
					return false
				}
			case httpMuxHandle:
				if len(callExpr.Args) == 2 {
					// Instrument handle funcs
					comment.Debug(manager.getDecoratorPackage(), stmt, "Wrapping http.Handle with newrelic.WrapHandle")
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

////////////////////////////
// Pre-Instrumentation Tracing Functions
////////////////////////////

func DetectWrappedRoutes(manager *InstrumentationManager, c *dstutil.Cursor) {
	mainFunctionNode := c.Node()
	if decl, ok := mainFunctionNode.(*dst.FuncDecl); ok {
		// Check if we're in the main function
		if decl.Name.Name != "main" {
			return
		}

		// Traverse the body of the main function
		dstutil.Apply(decl.Body, func(c *dstutil.Cursor) bool {
			node := c.Node()
			switch stmt := node.(type) {
			case *dst.ExprStmt:
				if callExpr, ok := stmt.X.(*dst.CallExpr); ok {
					if callIdent, ok := callExpr.Fun.(*dst.Ident); ok {
						if callIdent.Name != "HandleFunc" {
							break
						}
						for _, arg := range callExpr.Args {
							argExpr, ok := arg.(*dst.CallExpr)
							if !ok {
								continue
							}
							ident, ok := argExpr.Fun.(*dst.Ident)
							if !ok {
								continue
							}
							fmt.Println(ident)
							if ident.Name == "WrapHandleFunc" && ident.Path == "github.com/newrelic/go-agent/v3/newrelic" {
								funcName, ok := argExpr.Args[len(argExpr.Args)-1].(*dst.Ident)
								if !ok {
									continue
								}
								fun := manager.transactionCache.Functions[funcName.Name]
								if fun != nil {
									manager.transactionCache.AddFuncDecl(fun)
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
