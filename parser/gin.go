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
					if ident.Name == "Default" && ident.Path == GinImportPath {
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
			// Loop through the args and check for *gin.Context
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

func ginAnonymousFunction(node dst.Node, manager *InstrumentationManager) (*dst.FuncLit, bool, string) {
	anonFuncCount := 1
	switch v := node.(type) {
	case *dst.ExprStmt:
		if call, ok := v.X.(*dst.CallExpr); ok {
			if sel, ok := call.Fun.(*dst.SelectorExpr); ok {
				if ident, ok := sel.X.(*dst.Ident); ok {
					// Check if the GET call belongs to the gin router. Ensures no other GET functions are instrumented
					if sel.Sel.Name == "GET" && manager.facts.GetFact(ident.Name) == facts.GinRouter {
						anonFunctionRoute := ""
						for _, arg := range call.Args {
							// Get the route name from the anonymous function
							if util.TypeOf(arg, manager.getDecoratorPackage()).Underlying() == types.Typ[types.String] {
								anonFunctionRoute = arg.(*dst.BasicLit).Value
								anonFunctionRoute = anonFunctionRoute[:1] + anonFunctionRoute[2:]
							}
							if funcLit, ok := arg.(*dst.FuncLit); ok {
								// Check if the anonymous function has a single argument of type *gin.Context
								if len(funcLit.Type.Params.List) == 1 {
									if starExpr, ok := funcLit.Type.Params.List[0].Type.(*dst.StarExpr); ok {
										ctxName := funcLit.Type.Params.List[0].Names[0].Name
										if ident, ok := starExpr.X.(*dst.Ident); ok {
											if ident.Name == "Context" && ident.Path == GinImportPath {
												path := util.PackagePath(ident, manager.getDecoratorPackage())
												if path == codegen.GinImportPath {
													// If mulitple anonymous functions are present, append a number to the segment name
													if anonFuncCount > 1 {
														if len(anonFunctionRoute) > 1 && anonFunctionRoute[0] == '"' && anonFunctionRoute[len(anonFunctionRoute)-1] == '"' {
															anonFunctionRoute = anonFunctionRoute[1 : len(anonFunctionRoute)-1]
														}
														anonFunctionRoute = fmt.Sprintf("\"%s-%d\"", anonFunctionRoute, anonFuncCount)
													} else {
														comment.Warn(manager.getDecoratorPackage(), v, "Since the handler function name is used as the transaction name,", "anonymous functions do not get usefully named.", "We encourage transforming anonymous functions into named functions")
													}
													funcLit.Body.List = append([]dst.Stmt{codegen.TxnFromGinContext(defaultTxnName, ctxName), codegen.DeferStartSegment(defaultTxnName, anonFunctionRoute)}, funcLit.Body.List...)
													anonFuncCount++
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

		}
	}
	return nil, false, ""
}

func FindAnonymousFunctions(manager *InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	ginAnonymousFunction(currentNode, manager)
}

func InstrumentGinMiddleware(manager *InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	if call, ok, routerName := ginMiddlewareCall(currentNode); ok {
		err := manager.facts.AddFact(facts.Entry{
			Name: routerName,
			Fact: facts.GinRouter,
		})
		if err != nil {
			fmt.Println("Error adding fact: ", err)
		}

		c.InsertAfter(codegen.NrGinMiddleware(call, routerName, manager.agentVariableName))

	}
}

func InstrumentGinFunction(manager *InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	if ctxName, ok := ginFunctionCall(currentNode, manager.getDecoratorPackage()); ok {
		funcDecl := currentNode.(*dst.FuncDecl)
		decl, ok := TraceFunction(manager, funcDecl, TraceDownstreamFunction(defaultTxnName))
		if ok {
			decl.Body.List = append([]dst.Stmt{codegen.TxnFromGinContext(defaultTxnName, ctxName)}, decl.Body.List...)
			c.Replace(decl)
		}
	}
}
