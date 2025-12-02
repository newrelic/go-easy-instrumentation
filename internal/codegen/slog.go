package codegen

import (
	"github.com/dave/dst"
)

const (
	NrslogImportPath = "github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"
	SlogImportPath   = "log/slog"
)

// GinMiddlewareCall returns a new relic gin middleware call, and a string representing the import path
// of the library that contains the middleware function
func SlogHandlerWrapper(handlerName string) (*dst.ExprStmt, string) {
	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   &dst.Ident{Name: handlerName},
				Sel: &dst.Ident{Name: "Use"},
			},
			Args: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.Ident{
						Name: "Middleware",
						Path: NrslogImportPath,
					},
				},
			},
		},
	}, NrslogImportPath
}
