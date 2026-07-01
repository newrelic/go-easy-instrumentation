package nrpq

import (
	"go/token"

	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
)

// createContextWithTransaction returns: ctx := newrelic.NewContext(context.Background(), nrTxn)
func createContextWithTransaction(txnName string) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{dst.NewIdent(codegen.DefaultContextParameter)},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			codegen.NewContextExpression(
				&dst.CallExpr{Fun: &dst.Ident{Name: "Background", Path: "context"}},
				dst.NewIdent(txnName),
			),
		},
	}
}

