package nrpq

import (
	"go/token"

	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
)

// createContextWithTransaction returns: ctx := newrelic.NewContext(context.Background(), nrTxn)
func createContextWithTransaction(ctxName, txnName string) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{dst.NewIdent(ctxName)},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			codegen.NewContextExpression(
				&dst.CallExpr{Fun: &dst.Ident{Name: "Background", Path: "context"}},
				dst.NewIdent(txnName),
			),
		},
	}
}
