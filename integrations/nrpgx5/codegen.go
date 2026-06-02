package nrpgx5

import (
	"go/token"

	"github.com/dave/dst"
)

const (
	PgxImportPath     = "github.com/jackc/pgx/v5"
	PgxPoolImportPath = "github.com/jackc/pgx/v5/pgxpool"
	Nrpgx5ImportPath  = "github.com/newrelic/go-agent/v3/integrations/nrpgx5"
)

// CreatePgxParseConfig creates an assignment that parses a pgx connection string into a config.
// config, err := pgx.ParseConfig(connStr)
func CreatePgxParseConfig(configVar string, connStrExpr dst.Expr) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{dst.NewIdent(configVar), dst.NewIdent("err")},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun:  &dst.Ident{Name: "ParseConfig", Path: PgxImportPath},
				Args: []dst.Expr{dst.Clone(connStrExpr).(dst.Expr)},
			},
		},
	}
}

// CreatePgxPoolParseConfig creates an assignment that parses a pgxpool connection string into a config.
// config, err := pgxpool.ParseConfig(connStr)
func CreatePgxPoolParseConfig(configVar string, connStrExpr dst.Expr) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{dst.NewIdent(configVar), dst.NewIdent("err")},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun:  &dst.Ident{Name: "ParseConfig", Path: PgxPoolImportPath},
				Args: []dst.Expr{dst.Clone(connStrExpr).(dst.Expr)},
			},
		},
	}
}

// CreateTracerAssignment creates an assignment that injects the nrpgx5 tracer into a pgx config.
// config.Tracer = nrpgx5.NewTracer()
func CreateTracerAssignment(configVar string) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.SelectorExpr{
				X:   dst.NewIdent(configVar),
				Sel: dst.NewIdent("Tracer"),
			},
		},
		Tok: token.ASSIGN,
		Rhs: []dst.Expr{newTracerCall()},
	}
}

// CreatePoolTracerAssignment creates an assignment that injects the nrpgx5 tracer into a pgxpool config.
// config.ConnConfig.Tracer = nrpgx5.NewTracer()
func CreatePoolTracerAssignment(configVar string) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.SelectorExpr{
				X: &dst.SelectorExpr{
					X:   dst.NewIdent(configVar),
					Sel: dst.NewIdent("ConnConfig"),
				},
				Sel: dst.NewIdent("Tracer"),
			},
		},
		Tok: token.ASSIGN,
		Rhs: []dst.Expr{newTracerCall()},
	}
}

// CreatePgxConnectConfig creates an assignment that connects to a pgx database using a config.
// conn, err := pgx.ConnectConfig(ctx, config)
func CreatePgxConnectConfig(connVar string, ctxExpr dst.Expr, configVar string) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{dst.NewIdent(connVar), dst.NewIdent("err")},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{Name: "ConnectConfig", Path: PgxImportPath},
				Args: []dst.Expr{
					dst.Clone(ctxExpr).(dst.Expr),
					dst.NewIdent(configVar),
				},
			},
		},
	}
}

// CreatePgxPoolNewWithConfig creates an assignment that creates a pgxpool using a config.
// pool, err := pgxpool.NewWithConfig(ctx, config)
func CreatePgxPoolNewWithConfig(poolVar string, ctxExpr dst.Expr, configVar string) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{dst.NewIdent(poolVar), dst.NewIdent("err")},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{Name: "NewWithConfig", Path: PgxPoolImportPath},
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
