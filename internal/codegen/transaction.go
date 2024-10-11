package codegen

import (
	"fmt"
	"go/token"

	"github.com/dave/dst"
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

func TxnNewGoroutine(txnVarName string) *dst.CallExpr {
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X: &dst.Ident{
				Name: txnVarName,
			},
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

func NoticeUncheckedError(stmt dst.Stmt) {
	errList := []string{
		"// NR-WARNING: Unchecked Error, please consult New Relic documentation on error capture",
		"// https://docs.newrelic.com/docs/apm/agents/go-agent/api-guides/guide-using-go-agent-api/#errors",
	}

	if len(stmt.Decorations().Start) > 0 {
		errList = append(errList, "//")
	}
	stmt.Decorations().Start.Prepend(errList...)
}

func SuspectExpectedError(stmt dst.Stmt) {
	errList := []string{
		"// NR-INFO: Possible expected error detected: please consult New Relic documentation on expected errors to learn how to capture it",
		"// https://docs.newrelic.com/docs/apm/agents/go-agent/api-guides/guide-using-go-agent-api/#errors",
	}

	if len(stmt.Decorations().Start) > 0 {
		errList = append(errList, "//")
	}
	stmt.Decorations().Start.Prepend(errList...)
}

func UnknownError(stmt dst.Stmt) {
	errList := []string{
		"// NR-WARNING: Unable to determine how to automatically capture this error: please consult New Relic documentation on errors to manually capture it",
		"// https://docs.newrelic.com/docs/apm/agents/go-agent/api-guides/guide-using-go-agent-api/#errors",
	}

	if len(stmt.Decorations().Start) > 0 {
		errList = append(errList, "//")
	}
	stmt.Decorations().Start.Prepend(errList...)
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
