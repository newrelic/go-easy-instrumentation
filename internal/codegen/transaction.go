package codegen

import (
	"fmt"
	"go/token"

	"github.com/dave/dst"
)

const (
	DefaultTransactionVariable = "nrTxn"
)

func GetApplication(transactionVariableExpression dst.Expr) dst.Expr {
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   dst.Clone(transactionVariableExpression).(dst.Expr),
			Sel: dst.NewIdent("Application"),
		},
	}
}

func EndTransaction(transactionVariableName string) *dst.ExprStmt {
	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(transactionVariableName),
				Sel: dst.NewIdent("End"),
			},
		},
	}
}

// NewTransactionParameter returns a field definition for a transaction parameter
func NewTransactionParameter(txnName string) *dst.Field {
	return &dst.Field{
		Names: []*dst.Ident{
			{
				Name: txnName,
			},
		},
		Type: &dst.StarExpr{
			X: &dst.Ident{
				Name: "Transaction",
				Path: NewRelicAgentImportPath,
			},
		},
	}
}

// TxnNewGoroutine returns a call to txn.NewGoroutine()
func TxnNewGoroutine(transaction dst.Expr) *dst.CallExpr {
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X: dst.Clone(transaction).(dst.Expr),
			Sel: &dst.Ident{
				Name: "NewGoroutine",
			},
		},
	}
}

// starts a NewRelic transaction
// if overwireVariable is true, the transaction variable will be overwritten by variable assignment, otherwise it will be defined
func StartTransaction(appVariableName, transactionVariableName, transactionName string, overwriteVariable bool) *dst.AssignStmt {
	tok := token.DEFINE
	if overwriteVariable {
		tok = token.ASSIGN
	}
	return &dst.AssignStmt{
		Lhs: []dst.Expr{dst.NewIdent(transactionVariableName)},
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Args: []dst.Expr{
					&dst.BasicLit{
						Kind:  token.STRING,
						Value: fmt.Sprintf(`"%s"`, transactionName),
					},
				},
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent(appVariableName),
					Sel: dst.NewIdent("StartTransaction"),
				},
			},
		},
		Tok: tok,
	}
}

func TxnFromContext(txnVariable string, contextObject dst.Expr) *dst.AssignStmt {
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
					Name: "FromContext",
					Path: NewRelicAgentImportPath,
				},
				Args: []dst.Expr{
					dst.Clone(contextObject).(dst.Expr),
				},
			},
		},
	}
}

// TxnFromContextExpression returns a call to `newrelic.FromContext(contextObject)`
func TxnFromContextExpression(contextObject dst.Expr) dst.Expr {
	return &dst.CallExpr{
		Fun: &dst.Ident{
			Name: "FromContext",
			Path: NewRelicAgentImportPath,
		},
		Args: []dst.Expr{
			dst.Clone(contextObject).(dst.Expr),
		},
	}
}
