package codegen

import (
	"go/token"

	"github.com/dave/dst"
)

const (
	NrginImportPath = "github.com/newrelic/go-agent/v3/integrations/nrgin"
	GinImportPath   = "github.com/gin-gonic/gin"
)

func NrGinMiddleware(routerName string, agentVariableName dst.Expr) *dst.ExprStmt {
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
						agentVariableName,
					},
				},
			},
		},
	}
}
func TxnFromGinContext(txnVariable string, ctxName string) *dst.AssignStmt {
	return &dst.AssignStmt{
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
		Decs: dst.AssignStmtDecorations{
			NodeDecs: dst.NodeDecs{
				Before: dst.NewLine,
			},
		},
	}
}
