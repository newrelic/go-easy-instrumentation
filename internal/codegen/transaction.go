package codegen

import (
	"fmt"
	"go/token"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
)

const (
	DefaultTransactionVariable = "nrTxn"
)

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

func TxnAsParameter(txnName string) *dst.Field {
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
	txnClone := dst.Clone(transaction).(dst.Expr)
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X: txnClone,
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

func NoticeError(errExpr dst.Expr, txnName string, stmtBlock dst.Stmt) *dst.ExprStmt {
	var decs dst.ExprStmtDecorations
	// copy all decs below the current statement into this statement
	if stmtBlock != nil {
		decs.Before = stmtBlock.Decorations().Before
		decs.Start = stmtBlock.Decorations().Start
		stmtBlock.Decorations().Before = dst.None
		stmtBlock.Decorations().Start.Clear()
	}

	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X: &dst.Ident{
					Name: txnName,
				},
				Sel: &dst.Ident{
					Name: "NoticeError",
				},
			},
			Args: []dst.Expr{errExpr},
		},
		Decs: decs,
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

// returns true if a node contains a call to `txn.NewGoroutine()`
func ContainsTxnNewGoroutine(pkg *decorator.Package, node dst.Node) bool {
	found := false
	dst.Inspect(node, func(node dst.Node) bool {
		sel, ok := node.(*dst.SelectorExpr)
		t := util.TypeOf(sel.X, pkg)
		if ok && t != nil && sel.Sel.Name == "NewGoroutine" && t.String() == "*newrelic.Transaction" {
			found = true
			return false
		}

		return true
	})

	return found
}

// TransactionParameter returns a field definition for a function parameter that is a *newrelic.Transaction
func TransactionParameter(parameterName string) *dst.Field {
	return &dst.Field{
		Names: []*dst.Ident{
			{
				Name: parameterName,
			},
		},
		Type: &dst.StarExpr{
			X: &dst.Ident{
				Name: "Transaction",
				Path: "newrelic",
			},
		},
	}
}
