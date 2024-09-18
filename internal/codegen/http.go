package codegen

import (
	"go/token"

	"github.com/dave/dst"
)

const (
	HttpImportPath = "net/http"
)

func HttpRequestContext() dst.Expr {
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X: &dst.Ident{
				Name: "r",
			},
			Sel: &dst.Ident{
				Name: "Context",
			},
		},
	}
}

func RoundTripper(clientVariable dst.Expr, spacingAfter dst.SpaceType) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.SelectorExpr{
				X:   dst.Clone(clientVariable).(dst.Expr),
				Sel: dst.NewIdent("Transport"),
			},
		},
		Tok: token.ASSIGN,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "NewRoundTripper",
					Path: NewRelicAgentImportPath,
				},
				Args: []dst.Expr{
					&dst.SelectorExpr{
						X:   dst.Clone(clientVariable).(dst.Expr),
						Sel: dst.NewIdent("Transport"),
					},
				},
			},
		},
		Decs: dst.AssignStmtDecorations{
			NodeDecs: dst.NodeDecs{
				After: spacingAfter,
			},
		},
	}
}

// adds a transaction to the HTTP request context object by creating a line of code that injects it
// equal to calling: newrelic.RequestWithTransactionContext()
func WrapRequestContext(request dst.Expr, txnVar string, nodeDecs *dst.NodeDecs) *dst.AssignStmt {
	// Copy all decs above prior statement into this one
	decs := dst.AssignStmtDecorations{}
	if nodeDecs != nil {
		decs.NodeDecs = dst.NodeDecs{
			Before: nodeDecs.Before,
			Start:  nodeDecs.Start,
		}

		// Clear the decs from the previous node since they are being moved up
		nodeDecs.Before = dst.None
		nodeDecs.Start.Clear()
	}

	return &dst.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: []dst.Expr{dst.Clone(request).(dst.Expr)},
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "RequestWithTransactionContext",
					Path: NewRelicAgentImportPath,
				},
				Args: []dst.Expr{
					dst.Clone(request).(dst.Expr),
					dst.NewIdent(txnVar),
				},
			},
		},
		Decs: decs,
	}
}
