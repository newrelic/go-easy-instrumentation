package parser

import (
	"go/token"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
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

func InstrumentGinMiddleware(manager *InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	if call, ok, routerName := ginMiddlewareCall(currentNode); ok {
		c.InsertAfter(codegen.NrGinMiddleware(call, routerName, manager.agentVariableName))

	}
}

func InstrumentGinFunction(manager *InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	if ctxName, ok := ginFunctionCall(currentNode, manager.getDecoratorPackage()); ok {
		// cast the node to a *dst.FuncDecl
		funcDecl := currentNode.(*dst.FuncDecl)
		decl, ok := TraceFunction(manager, funcDecl, TraceDownstreamFunction(defaultTxnName))
		if ok {
			decl.Body.List = append([]dst.Stmt{TxnFromGinContext(defaultTxnName, ctxName)}, decl.Body.List...)
			c.Replace(decl)
		}
	}
}

func TxnFromGinContext(txnVariable string, ctxName string) *dst.AssignStmt {
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
					Name: "Transaction",
					Path: NrginImportPath,
				},
				Args: []dst.Expr{
					&dst.Ident{
						Name: ctxName,
					},
				},
			},
		},
	}
}
