package parser

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
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

func InstrumentGinMiddleware(manager *InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	if call, ok, routerName := ginMiddlewareCall(currentNode); ok {
		// Add test comment after teh call
		c.InsertAfter(codegen.NrGinMiddleware(call, routerName, manager.agentVariableName))

	}
}
