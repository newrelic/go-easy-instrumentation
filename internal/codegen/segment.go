package codegen

import (
	"fmt"
	"go/token"

	"github.com/dave/dst"
)

func DeferSegment(segmentName, txnVarName string) *dst.DeferStmt {
	return &dst.DeferStmt{
		Call: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: dst.NewIdent(txnVarName),
						Sel: &dst.Ident{
							Name: "StartSegment",
						},
					},
					Args: []dst.Expr{
						&dst.BasicLit{
							Kind:  token.STRING,
							Value: fmt.Sprintf(`"%s"`, segmentName),
						},
					},
				},
				Sel: &dst.Ident{
					Name: "End",
				},
			},
		},
	}
}

func StartExternalSegment(request dst.Expr, txnVar, segmentVar string, nodeDecs *dst.NodeDecs) *dst.AssignStmt {
	// copy all preceeding decorations from the previous node
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
		Tok: token.DEFINE,
		Lhs: []dst.Expr{
			dst.NewIdent(segmentVar),
		},
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "StartExternalSegment",
					Path: NewRelicAgentImportPath,
				},
				Args: []dst.Expr{
					dst.NewIdent(txnVar),
					dst.Clone(request).(dst.Expr),
				},
			},
		},
		Decs: decs,
	}
}

func EndExternalSegment(segmentName string, nodeDecs *dst.NodeDecs) *dst.ExprStmt {
	decs := dst.ExprStmtDecorations{}
	if nodeDecs != nil {
		decs.NodeDecs = dst.NodeDecs{
			After: nodeDecs.After,
			End:   nodeDecs.End,
		}

		nodeDecs.After = dst.None
		nodeDecs.End.Clear()
	}

	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(segmentName),
				Sel: dst.NewIdent("End"),
			},
		},
		Decs: decs,
	}
}

func CaptureHttpResponse(segmentVariable string, responseVariable dst.Expr) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.SelectorExpr{
				X:   dst.NewIdent(segmentVariable),
				Sel: dst.NewIdent("Response"),
			},
		},
		Rhs: []dst.Expr{
			dst.Clone(responseVariable).(dst.Expr),
		},
		Tok: token.ASSIGN,
	}
}
