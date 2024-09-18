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

func NoticeError(errExpr dst.Expr, txnName string, nodeDecs *dst.NodeDecs) *dst.ExprStmt {
	var decs dst.ExprStmtDecorations
	// copy all decs below the current statement into this statement
	if nodeDecs != nil {
		decs = dst.ExprStmtDecorations{
			NodeDecs: dst.NodeDecs{
				After: nodeDecs.After,
				End:   nodeDecs.End,
			},
		}

		// remove coppied decs from above node
		nodeDecs.After = dst.None
		nodeDecs.End.Clear()
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
