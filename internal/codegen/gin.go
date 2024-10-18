package codegen

import (
	"github.com/dave/dst"
)

const (
	NrginImportPath = "github.com/newrelic/go-agent/v3/integrations/nrgin"
	GinImportPath   = "github.com/gin-gonic/gin"
)

func NrGinMiddleware(call *dst.CallExpr, routerName string, agentVariableName string) *dst.ExprStmt {
	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   &dst.Ident{Name: routerName},
				Sel: &dst.Ident{Name: "Use"},
			},
			Args: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.Ident{
						Name: "Middleware",
						Path: NrginImportPath,
					},
					Args: []dst.Expr{
						&dst.Ident{Name: agentVariableName},
					},
				},
			},
		},
	}
}
