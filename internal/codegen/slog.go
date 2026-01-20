package codegen

import (
	"go/token"

	"github.com/dave/dst"
)

const (
	NrslogImportPath = "github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
	SlogImportPath   = "log/slog"
)

// TODO: SlogHandlerWrapper to correctly return nrslog.WrapHandler(app, handler)
// TODO: Detect slog.New(....) function call. Once we do, we can modify the argument to use our new wrapped handler from above ^

// SlogHandlerWrapper returns a New Relic  middleware call, and a string representing the import path
// of the library that contains the middleware function
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

	//	return &dst.ExprStmt{
	//		X: &dst.CallExpr{
	//			Fun: &dst.SelectorExpr{
	//				X:   &dst.Ident{Name: handlerName},
	//				Sel: &dst.Ident{Name: "Use"},
	//			},
	//			Args: []dst.Expr{
	//				&dst.CallExpr{
	//					Fun: &dst.Ident{
	//						Name: "Middleware",
	//						Path: NrslogImportPath,
	//					},
	//				},
	//			},
	//		},
	//	}, NrslogImportPath
}
