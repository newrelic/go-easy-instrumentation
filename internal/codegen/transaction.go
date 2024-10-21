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

func TxnNewGoroutine(transaction dst.Expr) *dst.CallExpr {
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X: transaction,
			Sel: &dst.Ident{
				Name: "NewGoroutine",
			},
		},
	}
}

// returns true if a node contains a call to `txn.NewGoroutine()`
func ContainsTxnNewGoroutine(node dst.Node) bool {
	ok := false
	dst.Inspect(node, func(node dst.Node) bool {
		call, ok := node.(*dst.CallExpr)
		if ok {
			sel, ok := call.Fun.(*dst.SelectorExpr)
			if ok {
				if sel.Sel.Name == "NewGoroutine" {
					ok = true
					return false
				}
			}
		}
		return true
	})

	return ok
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

// GetApplication returns an assignment statement that assigns the application from a transaction to a variable
// equivalent to `agent := txn.Application()`
func GetApplication(txn dst.Expr, agentVariableName string) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{dst.NewIdent(agentVariableName)},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   txn,
					Sel: dst.NewIdent("Application"),
				},
			},
		},
	}
}
