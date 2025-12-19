package codegen

import (
	"fmt"
	"go/token"

	"github.com/dave/dst"
)

// CreateSQLTransaction creates a transaction assignment for SQL operations
// nrTxn := app.StartTransaction("mySQL/QueryRow")
func CreateSQLTransaction(agentVarName, txnVarName, sqlMethodName string) *dst.AssignStmt {
	txnNameStr := fmt.Sprintf("mySQL/%s", sqlMethodName)

	return &dst.AssignStmt{
		Lhs: []dst.Expr{&dst.Ident{Name: txnVarName}},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: agentVarName},
					Sel: &dst.Ident{Name: "StartTransaction"},
				},
				Args: []dst.Expr{
					&dst.BasicLit{
						Kind:  token.STRING,
						Value: fmt.Sprintf(`"%s"`, txnNameStr),
					},
				},
			},
		},
	}
}

// CreateContextWithTransaction creates a context with an embedded transaction
// ctx := newrelic.NewContext(context.Background(), nrTxn)
func CreateContextWithTransaction(ctxName, txnName string) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{&dst.Ident{Name: ctxName}},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: "newrelic"},
					Sel: &dst.Ident{Name: "NewContext"},
				},
				Args: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "context"},
							Sel: &dst.Ident{Name: "Background"},
						},
					},
					&dst.Ident{Name: txnName},
				},
			},
		},
	}
}

// CreateTransactionEnd creates a transaction end statement
// nrTxn.End()
func CreateTransactionEnd(txnName string) *dst.ExprStmt {
	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   &dst.Ident{Name: txnName},
				Sel: &dst.Ident{Name: "End"},
			},
		},
	}
}
