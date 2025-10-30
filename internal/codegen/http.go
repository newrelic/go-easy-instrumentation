package codegen

import (
	"go/token"

	"github.com/dave/dst"
)

const (
	HttpImportPath = "net/http"
)

func HttpRequestContext(reqArgName string) dst.Expr {
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X: &dst.Ident{
				Name: reqArgName,
			},
			Sel: &dst.Ident{
				Name: "Context",
			},
		},
	}
}

// WrapHttpHandleFunc does an in place edit of a call expression to http.HandleFunc
// replacing it with a call to newrelic.WrapHandleFunc
//
// agentVariable should be passed from tracestate.State and WILL NOT BE CLONED
func WrapHttpHandleFunc(agentVariable dst.Expr, handle *dst.CallExpr) {
	oldArgs := handle.Args

	handle.Args = []dst.Expr{
		&dst.CallExpr{
			Fun: &dst.Ident{
				Name: "WrapHandleFunc",
				Path: NewRelicAgentImportPath,
			},
			Args: []dst.Expr{
				agentVariable,
				oldArgs[0],
				oldArgs[1],
			},
		},
	}
}

// WrapHttpHandle does an in place edit of a call expression to http.Handle
// replacing it with a call to newrelic.WrapHandle
//
// agentVariable should be passed from tracestate.State and WILL NOT BE CLONED
func WrapHttpHandle(agentVariable dst.Expr, handle *dst.CallExpr) {
	oldArgs := handle.Args

	handle.Args = []dst.Expr{
		&dst.CallExpr{
			Fun: &dst.Ident{
				Name: "WrapHandle",
				Path: NewRelicAgentImportPath,
			},
			Args: []dst.Expr{
				agentVariable,
				oldArgs[0],
				oldArgs[1],
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
func WrapRequestContext(request dst.Expr, txnVariable dst.Expr, nodeDecs *dst.NodeDecs) *dst.AssignStmt {
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
					txnVariable,
				},
			},
		},
		Decs: decs,
	}
}
