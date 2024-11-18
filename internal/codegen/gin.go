package codegen

import (
	"go/token"

	"github.com/dave/dst"
)

const (
	NrginImportPath = "github.com/newrelic/go-agent/v3/integrations/nrgin"
	GinImportPath   = "github.com/gin-gonic/gin"
)

func NrGinMiddleware(call *dst.CallExpr, routerName string, agentVariableName dst.Expr) *dst.ExprStmt {
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
func DeferStartSegment(txnVariable string, route string) *dst.DeferStmt {
	return &dst.DeferStmt{
		Call: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: txnVariable,
						},
						Sel: &dst.Ident{
							Name: "StartSegment",
						},
					},
					Args: []dst.Expr{
						&dst.BasicLit{
							Kind:  token.STRING,
							Value: route,
						},
					},
				},
				Sel: &dst.Ident{
					Name: "End",
				},
			},
		},
	}
}
