package codegen

import "github.com/dave/dst"

const (
	NrChiImportPath = "github.com/newrelic/go-agent/v3/integrations/nrgochi"
)

// Inject NR Middleware instrumentation logic to the Chi application via the `Use` directive.
// Ex:
//
//	router := chi.NewRouter()
//	router.Use(nrgochi.Middleware(app)) <--- Midddleware injection
func NrChiMiddleware(routerName string, agentVariableName dst.Expr) (*dst.ExprStmt, string) {
	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   &dst.Ident{Name: routerName},
				Sel: &dst.Ident{Name: "Use"},
			},
			Args: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.Ident{
						Name: "Middleware",
						Path: NrChiImportPath,
					},
					Args: []dst.Expr{
						agentVariableName,
					},
				},
			},
		},
	}, NrChiImportPath
}
