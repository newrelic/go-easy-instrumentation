package nrecho

import (
	"go/token"

	"github.com/dave/dst"
)

const (
	NrechoImportPath = "github.com/newrelic/go-agent/v3/integrations/nrecho-v4"
	EchoImportPath   = "github.com/labstack/echo/v4"
)

// NrEchoMiddleware returns an Echo middleware call that instruments the router
// with New Relic. Returns the middleware statement and the import path.
//
// Example output:
//
//	e.Use(nrecho.Middleware(app))
func NrEchoMiddleware(routerName string, agentVariableName dst.Expr) (*dst.ExprStmt, string) {
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
						Path: NrechoImportPath,
					},
					Args: []dst.Expr{
						agentVariableName,
					},
				},
			},
		},
	}, NrechoImportPath
}

// TxnFromEchoContext generates code to extract a New Relic transaction from
// an Echo context.
//
// Example output:
//
//	txn := nrecho.FromContext(c)
func TxnFromEchoContext(txnVariable string, ctxName string) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.Ident{Name: txnVariable},
		},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "FromContext",
					Path: NrechoImportPath,
				},
				Args: []dst.Expr{
					&dst.Ident{Name: ctxName},
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
