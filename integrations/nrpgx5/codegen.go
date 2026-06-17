package nrpgx5

import (
	"go/token"

	"github.com/dave/dst"
)

const (
	PgxImportPath     = "github.com/jackc/pgx/v5"
	PgxPoolImportPath = "github.com/jackc/pgx/v5/pgxpool"
	Nrpgx5ImportPath  = "github.com/newrelic/go-agent/v3/integrations/nrpgx5"

	// configVar is the name used for the synthesized *Config local. It is consistent
	// across pgx and pgxpool so that the same name appears in every replacement.
	configVar = "config"
)

// CreateParseConfig creates `config, err := <pkg>.ParseConfig(connStr)` where pkg is
// the package identified by importPath (PgxImportPath or PgxPoolImportPath).
func CreateParseConfig(connStrExpr dst.Expr, importPath string) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{dst.NewIdent(configVar), dst.NewIdent("err")},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun:  &dst.Ident{Name: "ParseConfig", Path: importPath},
				Args: []dst.Expr{dst.Clone(connStrExpr).(dst.Expr)},
			},
		},
	}
}

// CreateTracerAssignment creates `<receiver>.Tracer = nrpgx5.NewTracer()`.
// receiver selects the field that holds the tracer hook; pass dst.NewIdent("config")
// for pgx, or config.ConnConfig for pgxpool.
func CreateTracerAssignment(receiver dst.Expr) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.SelectorExpr{
				X:   receiver,
				Sel: dst.NewIdent("Tracer"),
			},
		},
		Tok: token.ASSIGN,
		Rhs: []dst.Expr{newTracerCall()},
	}
}

// CreateConnectWithConfig creates `<varName>, err := <fun>(ctx, config)`. fun is the
// connect-from-config call expression — pgx.ConnectConfig or pgxpool.NewWithConfig.
func CreateConnectWithConfig(varName string, ctxExpr dst.Expr, fun dst.Expr) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{dst.NewIdent(varName), dst.NewIdent("err")},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: fun,
				Args: []dst.Expr{
					dst.Clone(ctxExpr).(dst.Expr),
					dst.NewIdent(configVar),
				},
			},
		},
	}
}

// newTracerCall returns a nrpgx5.NewTracer() call expression.
func newTracerCall() *dst.CallExpr {
	return &dst.CallExpr{
		Fun: &dst.Ident{Name: "NewTracer", Path: Nrpgx5ImportPath},
	}
}
