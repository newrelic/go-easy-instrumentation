package nrslog

import (
	"go/token"

	"github.com/dave/dst"
)

const (
	NrslogImportPath = "github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
	SlogImportPath   = "log/slog"
)

func SlogHandlerWrapper(handlerName, nrHandlerName string) (*dst.AssignStmt, string) {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.Ident{
				Name: nrHandlerName,
			},
		},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "WrapHandler",
					Path: NrslogImportPath,
				},
				Args: []dst.Expr{
					&dst.Ident{
						Name: "NewRelicAgent",
					},
					&dst.Ident{
						Name: handlerName,
					},
				},
			},
		},
		Decs: dst.AssignStmtDecorations{
			NodeDecs: dst.NodeDecs{
				Before: dst.NewLine,
			},
		},
	}, NrslogImportPath
}
