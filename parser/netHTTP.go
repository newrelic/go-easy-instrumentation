package main

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
)

const (
	netHttpPath = "net/http"

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

func typeOfIdent(ident *dst.Ident, pkg *decorator.Package) string {
	if ident == nil || pkg == nil {
		return ""
	}
	astNode := pkg.Decorator.Ast.Nodes[ident]
	var astIdent *ast.Ident
	switch v := astNode.(type) {
	case *ast.SelectorExpr:
		if v != nil {
			astIdent = v.Sel
		}
	case *ast.Ident:
		astIdent = v
	default:
		return ""
	}

	if pkg.TypesInfo != nil {
		uses, ok := pkg.TypesInfo.Uses[astIdent]
		if ok {
			if uses.Pkg() != nil {
				return uses.Pkg().Path()
			}
		}
	}
	return ""
}

// GetNetHttpClientVariableName looks for an http client in the call expression n. If it finds one, the name
// of the variable containing the client will be returned as a string.
func GetNetHttpClientVariableName(n *dst.CallExpr, pkg *decorator.Package) string {
	if n == nil {
		return ""
	}

	Sel, ok := n.Fun.(*dst.SelectorExpr)
	if ok {
		switch v := Sel.X.(type) {
		case *dst.SelectorExpr:
			path := typeOfIdent(v.Sel, pkg)
			if path == netHttpPath {
				return v.Sel.Name
			}
		case *dst.Ident:
			path := typeOfIdent(v, pkg)
			if path == netHttpPath {
				return v.Name
			}
		}
	}
	return ""
}

// GetNetHttpMethod gets an http method if one is invoked in the call expression n, and returns the name of it as a string
func GetNetHttpMethod(n *dst.CallExpr, pkg *decorator.Package) string {
	if n == nil {
		return ""
	}

	switch v := n.Fun.(type) {
	case *dst.SelectorExpr:
		path := typeOfIdent(v.Sel, pkg)
		if path == netHttpPath {
			return v.Sel.Name
		}
	case *dst.Ident:
		path := typeOfIdent(v, pkg)
		if path == netHttpPath {
			return v.Name
		}
	}

	return ""
}

// WrapHandleFunc looks for an instance of http.HandleFunc() and wraps it with a new relic transaction
func WrapHandleFunc(n dst.Node, manager *InstrumentationManager, c *dstutil.Cursor) {
	callExpr, ok := n.(*dst.CallExpr)
	if ok {
		funcName := GetNetHttpMethod(callExpr, manager.GetDecoratorPackage())
		switch funcName {
		case httpHandleFunc, httpMuxHandle:
			if len(callExpr.Args) == 2 {
				// Instrument handle funcs
				oldArgs := callExpr.Args
				callExpr.Args = []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "WrapHandleFunc",
							Path: newrelicAgentImport,
						},
						Args: []dst.Expr{
							&dst.Ident{
								Name: manager.agentVariableName,
							},
							oldArgs[0],
							oldArgs[1],
						},
					},
				}
			}
		}
	}
}

func txnFromContext(txnVariable string) *dst.AssignStmt {
	return &dst.AssignStmt{
		Decs: dst.AssignStmtDecorations{
			NodeDecs: dst.NodeDecs{
				After: dst.EmptyLine,
			},
		},
		Lhs: []dst.Expr{
			&dst.Ident{
				Name: txnVariable,
			},
		},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "FromContext",
					Path: newrelicAgentImport,
				},
				Args: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X: &dst.Ident{
								Name: "r",
							},
							Sel: &dst.Ident{
								Name: "Context",
							},
						},
					},
				},
			},
		},
	}
}

// txnFromCtx injects a line of code that extracts a transaction from the context into the body of a function
func defineTxnFromCtx(fn *dst.FuncDecl, txnVariable string) {
	stmts := make([]dst.Stmt, len(fn.Body.List)+1)
	stmts[0] = txnFromContext(txnVariable)
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

// Recognize if a function is a handler func based on its contents, and inject instrumentation.
// This function discovers entrypoints to tracing for a given transaction and should trace all the way
// down the call chain of the function it is invoked on.
func InstrumentHandleFunction(n dst.Node, manager *InstrumentationManager, c *dstutil.Cursor) {
	fn, isFn := n.(*dst.FuncDecl)
	if isFn && isHttpHandler(fn, manager.GetDecoratorPackage()) {
		txnName := "nrTxn"
		newFn, ok := TraceFunction(manager, fn, txnName)
		if ok {
			defineTxnFromCtx(newFn, txnName)
			c.Replace(newFn)
			manager.UpdateFunctionDeclaration(newFn)
		}
	}
}

func injectRoundTripper(clientVariable dst.Expr, spacingAfter dst.SpaceType) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.SelectorExpr{
				X:   dst.Clone(clientVariable).(dst.Expr),
				Sel: dst.NewIdent("Transport"),
			},
		},
		Tok: token.ASSIGN,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "NewRoundTripper",
					Path: newrelicAgentImport,
				},
				Args: []dst.Expr{
					&dst.SelectorExpr{
						X:   dst.Clone(clientVariable).(dst.Expr),
						Sel: dst.NewIdent("Transport"),
					},
				},
			},
		},
		Decs: dst.AssignStmtDecorations{
			NodeDecs: dst.NodeDecs{
				After: spacingAfter,
			},
		},
	}
}

// more unit test friendly helper function
func isNetHttpClientDefinition(stmt *dst.AssignStmt) bool {
	if len(stmt.Rhs) == 1 && len(stmt.Lhs) == 1 && stmt.Tok == token.DEFINE {
		unary, ok := stmt.Rhs[0].(*dst.UnaryExpr)
		if ok && unary.Op == token.AND {
			lit, ok := unary.X.(*dst.CompositeLit)
			if ok {
				ident, ok := lit.Type.(*dst.Ident)
				if ok && ident.Name == "Client" && ident.Path == netHttpPath {
					return true
				}
			}
		}
	}
	return false
}

// InstrumentHttpClient automatically injects a newrelic roundtripper into any newly created http client
// looks for the following pattern: client := &http.Client{}
func InstrumentHttpClient(n dst.Node, manager *InstrumentationManager, c *dstutil.Cursor) {
	stmt, ok := n.(*dst.AssignStmt)
	if ok && isNetHttpClientDefinition(stmt) && c.Index() >= 0 && n.Decorations() != nil {
		c.InsertAfter(injectRoundTripper(stmt.Lhs[0], n.Decorations().After)) // add roundtripper to transports
		stmt.Decs.After = dst.None
		manager.AddImport(newrelicAgentImport)
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
				if ok && ident.Path == netHttpPath {
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
func CannotInstrumentHttpMethod(n dst.Node, manager *InstrumentationManager, c *dstutil.Cursor) {
	funcName, ok := isNetHttpMethodCannotInstrument(n)
	if ok {
		if decl := n.Decorations(); decl != nil {
			decl.Start.Prepend(cannotTraceOutboundHttp(funcName, n.Decorations())...)
		}
	}
}

func startExternalSegment(request dst.Expr, txnVar, segmentVar string, nodeDecs *dst.NodeDecs) *dst.AssignStmt {
	// copy all preceeding decorations from the previous node
	decs := dst.AssignStmtDecorations{}
	if nodeDecs != nil {
		decs.NodeDecs = dst.NodeDecs{
			Before: nodeDecs.Before,
			Start:  nodeDecs.Start,
		}

		// Clear the decs from the previous node since they are being moved up
		if nodeDecs != nil {
			nodeDecs.Before = dst.None
			nodeDecs.Start.Clear()
		}
	}

	return &dst.AssignStmt{
		Tok: token.DEFINE,
		Lhs: []dst.Expr{
			dst.NewIdent(segmentVar),
		},
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "StartExternalSegment",
					Path: newrelicAgentImport,
				},
				Args: []dst.Expr{
					dst.NewIdent(txnVar),
					dst.Clone(request).(dst.Expr),
				},
			},
		},
		Decs: decs,
	}
}

func captureHttpResponse(segmentVariable string, responseVariable dst.Expr) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.SelectorExpr{
				X:   dst.NewIdent(segmentVariable),
				Sel: dst.NewIdent("Response"),
			},
		},
		Rhs: []dst.Expr{
			dst.Clone(responseVariable).(dst.Expr),
		},
		Tok: token.ASSIGN,
	}
}

func endExternalSegment(segmentName string, nodeDecs *dst.NodeDecs) *dst.ExprStmt {
	decs := dst.ExprStmtDecorations{}
	if nodeDecs != nil {
		decs.NodeDecs = dst.NodeDecs{
			After: nodeDecs.After,
			End:   nodeDecs.End,
		}

		nodeDecs.After = dst.None
		nodeDecs.End.Clear()
	}

	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(segmentName),
				Sel: dst.NewIdent("End"),
			},
		},
		Decs: decs,
	}
}

// adds a transaction to the HTTP request context object by creating a line of code that injects it
// equal to calling: newrelic.RequestWithTransactionContext()
func addTxnToRequestContext(request dst.Expr, txnVar string, nodeDecs *dst.NodeDecs) *dst.AssignStmt {
	// Copy all decs above prior statement into this one
	decs := dst.AssignStmtDecorations{}
	if nodeDecs != nil {
		decs.NodeDecs = dst.NodeDecs{
			Before: nodeDecs.Before,
			Start:  nodeDecs.Start,
		}

		// Clear the decs from the previous node since they are being moved up
		nodeDecs.Before = dst.None
		nodeDecs.Start.Clear()
	}

	return &dst.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: []dst.Expr{dst.Clone(request).(dst.Expr)},
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "RequestWithTransactionContext",
					Path: newrelicAgentImport,
				},
				Args: []dst.Expr{
					dst.Clone(request).(dst.Expr),
					dst.NewIdent(txnVar),
				},
			},
		},
		Decs: decs,
	}
}

// getHttpResponseVariable returns the expression that contains an object of `*net/http.Response` type
func getHttpResponseVariable(manager *InstrumentationManager, stmt dst.Stmt) dst.Expr {
	var expression dst.Expr
	pkg := manager.GetDecoratorPackage()
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
func ExternalHttpCall(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, txnName string) bool {
	if c.Index() < 0 {
		return false
	}
	pkg := manager.GetDecoratorPackage()
	var call *dst.CallExpr
	dst.Inspect(stmt, func(n dst.Node) bool {
		switch v := n.(type) {
		case *dst.CallExpr:
			if GetNetHttpMethod(v, pkg) == httpDo {
				call = v
				return false
			}
		}
		return true
	})
	if call != nil && c.Index() >= 0 {
		clientVar := GetNetHttpClientVariableName(call, pkg)
		requestObject := call.Args[0]
		if clientVar == httpDefaultClientVariable {
			// create external segment to wrap calls made with default client
			segmentName := "externalSegment"
			c.InsertBefore(startExternalSegment(requestObject, txnName, segmentName, stmt.Decorations()))
			c.InsertAfter(endExternalSegment(segmentName, stmt.Decorations()))
			responseVar := getHttpResponseVariable(manager, stmt)
			manager.AddImport(newrelicAgentImport)
			if responseVar != nil {
				c.InsertAfter(captureHttpResponse(segmentName, responseVar))
			}
			return true
		} else {
			c.InsertBefore(addTxnToRequestContext(requestObject, txnName, stmt.Decorations()))
			manager.AddImport(newrelicAgentImport)
			return true
		}
	}
	return false
}

// WrapHandleFunction is a function that wraps net/http.HandeFunc() declarations inside of functions
// that are being traced by a transaction.
func WrapNestedHandleFunction(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, txnName string) bool {
	wasModified := false
	pkg := manager.GetDecoratorPackage()
	dst.Inspect(stmt, func(n dst.Node) bool {
		switch v := n.(type) {
		case *dst.CallExpr:
			callExpr := v
			funcName := GetNetHttpMethod(callExpr, pkg)
			switch funcName {
			case httpHandleFunc, httpMuxHandle:
				if len(callExpr.Args) == 2 {
					// Instrument handle funcs
					oldArgs := callExpr.Args
					callExpr.Args = []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.Ident{
								Name: "WrapHandleFunc",
								Path: newrelicAgentImport,
							},
							Args: []dst.Expr{
								&dst.CallExpr{
									Fun: &dst.SelectorExpr{
										X:   dst.NewIdent(txnName),
										Sel: dst.NewIdent("Application"),
									},
								},
								oldArgs[0],
								oldArgs[1],
							},
						},
					}
					wasModified = true
					manager.AddImport(newrelicAgentImport)
					return false
				}
			}
		}
		return true
	})

	return wasModified
}
