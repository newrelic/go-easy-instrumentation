package parser

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
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
	stmts[0] = codegen.TxnFromContext(txnVariable, codegen.HttpRequestContext())
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
		txnName := defaultTxnName
		newFn, ok := TraceFunction(manager, fn, TraceDownstreamFunction(txnName), noSegment())
		if ok {
			c.Replace(newFn)
			defineTxnFromCtx(newFn, txnName)
			manager.updateFunctionDeclaration(newFn)
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
		case *dst.AssignStmt:
			for _, expr := range v.Lhs {
				astExpr := pkg.Decorator.Ast.Nodes[expr].(ast.Expr)
				t := pkg.TypesInfo.TypeOf(astExpr).String()
				if t == "*net/http.Response" {
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
func ExternalHttpCall(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracingState) bool {
	if c.Index() < 0 {
		return false
	}
	pkg := manager.getDecoratorPackage()
	var call *dst.CallExpr
	dst.Inspect(stmt, func(n dst.Node) bool {
		switch v := n.(type) {
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
			c.InsertBefore(codegen.StartExternalSegment(requestObject, tracing.txnVariable, segmentName, stmt.Decorations()))
			c.InsertAfter(codegen.EndExternalSegment(segmentName, stmt.Decorations()))
			responseVar := getHttpResponseVariable(manager, stmt)
			manager.addImport(codegen.NewRelicAgentImportPath)
			if responseVar != nil {
				c.InsertAfter(codegen.CaptureHttpResponse(segmentName, responseVar))
			}
			return true
		} else {
			c.InsertBefore(codegen.WrapRequestContext(requestObject, tracing.txnVariable, stmt.Decorations()))
			manager.addImport(codegen.NewRelicAgentImportPath)
			return true
		}
	}
	return false
}

// WrapHandleFunction is a function that wraps net/http.HandeFunc() declarations inside of functions
// that are being traced by a transaction.
func WrapNestedHandleFunction(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracingState) bool {
	wasModified := false
	pkg := manager.getDecoratorPackage()
	dst.Inspect(stmt, func(n dst.Node) bool {
		switch v := n.(type) {
		case *dst.CallExpr:
			callExpr := v
			funcName := getNetHttpMethod(callExpr, pkg)
			switch funcName {
			case httpHandleFunc, httpMuxHandle:
				if len(callExpr.Args) == 2 {
					// Instrument handle funcs
					oldArgs := callExpr.Args
					if tracing.GetAgentVariable() != "" {
						callExpr.Args = []dst.Expr{
							&dst.CallExpr{
								Fun: &dst.Ident{
									Name: "WrapHandleFunc",
									Path: codegen.NewRelicAgentImportPath,
								},
								Args: []dst.Expr{
									&dst.Ident{
										Name: tracing.GetAgentVariable(),
									},
									oldArgs[0],
									oldArgs[1],
								},
							},
						}
					} else {
						callExpr.Args = []dst.Expr{
							&dst.CallExpr{
								Fun: &dst.Ident{
									Name: "WrapHandleFunc",
									Path: codegen.NewRelicAgentImportPath,
								},
								Args: []dst.Expr{
									&dst.CallExpr{
										Fun: &dst.SelectorExpr{
											X:   dst.NewIdent(tracing.GetTransactionVariable()),
											Sel: dst.NewIdent("Application"),
										},
									},
									oldArgs[0],
									oldArgs[1],
								},
							},
						}
					}
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
