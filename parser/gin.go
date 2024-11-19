package parser

import (
	"fmt"
	"go/types"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
	"github.com/newrelic/go-easy-instrumentation/parser/facts"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

const (
	NrginImportPath                = "github.com/newrelic/go-agent/v3/integrations/nrgin"
	GinImportPath                  = "github.com/gin-gonic/gin"
	NewRelicAgentImportPath string = "github.com/newrelic/go-agent/v3/newrelic"
)

func ginMiddlewareCall(node dst.Node) (*dst.CallExpr, bool, string) {
	switch v := node.(type) {
	case *dst.AssignStmt:
		if len(v.Rhs) == 1 {
			if call, ok := v.Rhs[0].(*dst.CallExpr); ok {
				if ident, ok := call.Fun.(*dst.Ident); ok {
					if ident.Name == "Default" || ident.Name == "New" && ident.Path == GinImportPath {
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

func ginFunctionCall(node dst.Node, pkg *decorator.Package) (string, bool) {
	switch v := node.(type) {
	case *dst.FuncDecl:
		if v.Name != nil {
			// Loop through the args and check for gin.Context
			for _, arg := range v.Type.Params.List {
				if len(arg.Names) != 1 {
					return "", false
				}
				ctxName := arg.Names[0].Name
				if ident, ok := arg.Type.(*dst.StarExpr); ok {
					if argument, ok := ident.X.(*dst.Ident); ok {
						if argument.Name == "Context" && argument.Path == GinImportPath {
							path := util.PackagePath(argument, pkg)
							if path == codegen.GinImportPath {
								return ctxName, true
							}
						}
					}
				}
			}
		}
	}
	return "", false
}

func checkForGinContext(funcLit *dst.FuncLit, manager *InstrumentationManager) (string, bool) {
	if starExpr, ok := funcLit.Type.Params.List[0].Type.(*dst.StarExpr); ok {
		ctxName := funcLit.Type.Params.List[0].Names[0].Name
		if ident, ok := starExpr.X.(*dst.Ident); ok {
			if ident.Name == "Context" && ident.Path == GinImportPath {
				path := util.PackagePath(ident, manager.getDecoratorPackage())
				if path == codegen.GinImportPath {
					return ctxName, true
				}
			}
		}
	}
	return "", false
}

// Casts the anonymous function as a call expression. This allows us to loop through the arguments to capture route names
func isGinRoute(v *dst.ExprStmt, manager *InstrumentationManager) (*dst.CallExpr, bool) {
	if call, ok := v.X.(*dst.CallExpr); ok {
		if sel, ok := call.Fun.(*dst.SelectorExpr); ok {
			if ident, ok := sel.X.(*dst.Ident); ok {
				// Check if the GET call belongs to the gin router. Ensures no other GET functions are instrumented
				if sel.Sel.Name == "GET" || sel.Sel.Name == "POST" || sel.Sel.Name == "PUT" || sel.Sel.Name == "DELETE" && manager.facts.GetFact(ident.Name) == facts.GinRouter {
					return call, true
				}
			}
		}
	}
	return nil, false
}
func ginAnonymousFunction(node dst.Node, manager *InstrumentationManager) (*dst.FuncLit, bool, string) {
	anonFuncCount := 1
	switch v := node.(type) {
	case *dst.ExprStmt:
		if call, ok := isGinRoute(v, manager); ok {
			anonFunctionRoute := ""
			for _, arg := range call.Args {
				// Get the route name from the anonymous function
				if util.TypeOf(arg, manager.getDecoratorPackage()).Underlying() == types.Typ[types.String] {
					anonFunctionRoute = arg.(*dst.BasicLit).Value
				}
				// If the argument is a function literal, we need to add instrumentation
				if funcLit, ok := arg.(*dst.FuncLit); ok {
					ctxName, isGinFunc := checkForGinContext(funcLit, manager)
					if isGinFunc {
						// If mulitple anonymous functions are present, append a number to the segment name so the user can have unique names for each segment
						if anonFuncCount > 1 {
							if len(anonFunctionRoute) > 1 && anonFunctionRoute[0] == '"' && anonFunctionRoute[len(anonFunctionRoute)-1] == '"' {
								anonFunctionRoute = anonFunctionRoute[1 : len(anonFunctionRoute)-1]
							}
							anonFunctionRoute = fmt.Sprintf("\"%s-%d\"", anonFunctionRoute, anonFuncCount)
						} else if !manager.anonymousFunctionWarning {
							manager.anonymousFunctionWarning = true
							comment.Warn(manager.getDecoratorPackage(), v, "Since the handler function name is used as the transaction name,", "anonymous functions do not get usefully named.", "We encourage transforming anonymous functions into named functions")
						}
						txnName := codegen.DefaultTransactionVariable

						funcLit.Body.List = append([]dst.Stmt{codegen.TxnFromGinContext(txnName, ctxName), codegen.DeferStartSegment(txnName, anonFunctionRoute)}, funcLit.Body.List...)
						anonFuncCount++
					}
				}
			}
		}
	}
	return nil, false, ""
}

func FindAnonymousFunctions(manager *InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	ginAnonymousFunction(currentNode, manager)
}

func InstrumentGinMiddleware(manager *InstrumentationManager, c *dstutil.Cursor) {
	mainFunctionNode := c.Node()
	if decl, ok := mainFunctionNode.(*dst.FuncDecl); ok {
		// only inject go agent into the main.main function
		if decl.Name.Name == "main" {
			// Loop through all the statements in the main function
			state := tracestate.Main(manager.agentVariableName)

			for i, stmt := range decl.Body.List {
				// Check if any return true for ginMiddlewareCall
				if call, ok, routerName := ginMiddlewareCall(stmt); ok {
					// Append at the current stmt location
					decl.Body.List = append(decl.Body.List[:i+1], append([]dst.Stmt{codegen.NrGinMiddleware(call, routerName, state.AgentVariable())}, decl.Body.List[i+1:]...)...)
					return
				}
			}

		} else {
			for i, stmt := range decl.Body.List {
				state := tracestate.FunctionBody(codegen.DefaultTransactionVariable)
				// Check if any return true for ginMiddlewareCall
				if call, ok, routerName := ginMiddlewareCall(stmt); ok {
					decl.Body.List = append(decl.Body.List[:i+1], append([]dst.Stmt{codegen.NrGinMiddleware(call, routerName, state.AgentVariable())}, decl.Body.List[i+1:]...)...)
					return
				}
			}

		}
	}
}

// txnFromCtx injects a line of code that extracts a transaction from the context into the body of a function
func defineTxnFromGinCtx(fn *dst.FuncDecl, txnVariable string, ctxName string) {
	stmts := make([]dst.Stmt, len(fn.Body.List)+1)
	stmts[0] = codegen.TxnFromGinContext(txnVariable, ctxName)
	for i, stmt := range fn.Body.List {
		stmts[i+1] = stmt
	}
	fn.Body.List = stmts
}

func InstrumentGinFunction(manager *InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	if ctxName, ok := ginFunctionCall(currentNode, manager.getDecoratorPackage()); ok {
		funcDecl := currentNode.(*dst.FuncDecl)
		txnName := codegen.DefaultTransactionVariable

		_, ok := TraceFunction(manager, funcDecl, tracestate.FunctionBody(txnName))
		if ok {
			defineTxnFromGinCtx(funcDecl, txnName, ctxName)
		}
	}
}
