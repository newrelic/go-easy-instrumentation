package nrpgx5

import (
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/parser"
	"github.com/stretchr/testify/assert"
)

func TestInstrumentPgxHandler(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "instrument pgx.Connect",
			// Explicit alias is required in test sources because CreateTestApp uses a minimal
			// go.mod without pgx/v5 as a dependency. Without type resolution, DST infers
			// package names from the last import path element, which is "v5" not "pgx".
			// The explicit alias tells DST unambiguously that "pgx" = github.com/jackc/pgx/v5.
			code: `package main

import (
	"context"
	pgx "github.com/jackc/pgx/v5"
)

func main() {
	conn, err := pgx.Connect(context.Background(), "postgres://user:pass@localhost/mydb")
	if err != nil {
		panic(err)
	}
	_ = conn
}
`,
			expect: `package main

import (
	"context"

	pgx "github.com/jackc/pgx/v5"
	"github.com/newrelic/go-agent/v3/integrations/nrpgx5"
)

func main() {
	config, err := pgx.ParseConfig("postgres://user:pass@localhost/mydb")
	config.Tracer = nrpgx5.NewTracer()
	conn, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		panic(err)
	}
	_ = conn
}
`,
		},
		{
			name: "instrument pgxpool.New",
			code: `package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://user:pass@localhost/mydb")
	if err != nil {
		panic(err)
	}
	_ = pool
}
`,
			expect: `package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/newrelic/go-agent/v3/integrations/nrpgx5"
)

func main() {
	config, err := pgxpool.ParseConfig("postgres://user:pass@localhost/mydb")
	config.ConnConfig.Tracer = nrpgx5.NewTracer()
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		panic(err)
	}
	_ = pool
}
`,
		},
		{
			name: "skip already instrumented pgx.Connect",
			code: `package main

import (
	"context"
	pgx "github.com/jackc/pgx/v5"
	"github.com/newrelic/go-agent/v3/integrations/nrpgx5"
)

func main() {
	config, err := pgx.ParseConfig("postgres://user:pass@localhost/mydb")
	config.Tracer = nrpgx5.NewTracer()
	conn, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		panic(err)
	}
	_ = conn
}
`,
			expect: `package main

import (
	"context"
	pgx "github.com/jackc/pgx/v5"
	"github.com/newrelic/go-agent/v3/integrations/nrpgx5"
)

func main() {
	config, err := pgx.ParseConfig("postgres://user:pass@localhost/mydb")
	config.Tracer = nrpgx5.NewTracer()
	conn, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		panic(err)
	}
	_ = conn
}
`,
		},
		{
			name: "skip already instrumented pgxpool.New",
			code: `package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/newrelic/go-agent/v3/integrations/nrpgx5"
)

func main() {
	config, err := pgxpool.ParseConfig("postgres://user:pass@localhost/mydb")
	config.ConnConfig.Tracer = nrpgx5.NewTracer()
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		panic(err)
	}
	_ = pool
}
`,
			expect: `package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/newrelic/go-agent/v3/integrations/nrpgx5"
)

func main() {
	config, err := pgxpool.ParseConfig("postgres://user:pass@localhost/mydb")
	config.ConnConfig.Tracer = nrpgx5.NewTracer()
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		panic(err)
	}
	_ = pool
}
`,
		},
		{
			name: "instrument pgx.Connect in function literal",
			code: `package main

import (
	"context"
	pgx "github.com/jackc/pgx/v5"
)

func main() {
	connect := func() {
		conn, err := pgx.Connect(context.Background(), "postgres://user:pass@localhost/mydb")
		if err != nil {
			panic(err)
		}
		_ = conn
	}
	connect()
}
`,
			expect: `package main

import (
	"context"

	pgx "github.com/jackc/pgx/v5"
	"github.com/newrelic/go-agent/v3/integrations/nrpgx5"
)

func main() {
	connect := func() {
		config, err := pgx.ParseConfig("postgres://user:pass@localhost/mydb")
		config.Tracer = nrpgx5.NewTracer()
		conn, err := pgx.ConnectConfig(context.Background(), config)
		if err != nil {
			panic(err)
		}
		_ = conn
	}
	connect()
}
`,
		},
		{
			name: "non-pgx function call is not instrumented",
			code: `package main

func main() {
	x, err := someFunc(ctx, "arg")
	_ = x
	_ = err
}
`,
			expect: `package main

func main() {
	x, err := someFunc(ctx, "arg")
	_ = x
	_ = err
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, InstrumentPgxHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestDetectPgxCallPattern(t *testing.T) {
	tests := []struct {
		name       string
		stmt       dst.Stmt
		importPath string
		methodName string
		wantVar    string
	}{
		{
			name: "detect pgx.Connect call",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "conn"},
					&dst.Ident{Name: "err"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{Name: "Connect", Path: PgxImportPath},
						Args: []dst.Expr{
							&dst.Ident{Name: "ctx"},
							&dst.BasicLit{Value: `"postgres://localhost/mydb"`},
						},
					},
				},
			},
			importPath: PgxImportPath,
			methodName: "Connect",
			wantVar:    "conn",
		},
		{
			name: "detect pgxpool.New call",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "pool"},
					&dst.Ident{Name: "err"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{Name: "New", Path: PgxPoolImportPath},
						Args: []dst.Expr{
							&dst.Ident{Name: "ctx"},
							&dst.BasicLit{Value: `"postgres://localhost/mydb"`},
						},
					},
				},
			},
			importPath: PgxPoolImportPath,
			methodName: "New",
			wantVar:    "pool",
		},
		{
			name: "wrong import path is not detected",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{&dst.Ident{Name: "conn"}, &dst.Ident{Name: "err"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{Name: "Connect", Path: "github.com/jackc/pgx/v4"},
						Args: []dst.Expr{
							&dst.Ident{Name: "ctx"},
							&dst.BasicLit{Value: `"postgres://localhost/mydb"`},
						},
					},
				},
			},
			importPath: PgxImportPath,
			methodName: "Connect",
			wantVar:    "",
		},
		{
			name: "wrong method name is not detected",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{&dst.Ident{Name: "conn"}, &dst.Ident{Name: "err"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{Name: "ConnectConfig", Path: PgxImportPath},
						Args: []dst.Expr{
							&dst.Ident{Name: "ctx"},
							&dst.BasicLit{Value: `"postgres://localhost/mydb"`},
						},
					},
				},
			},
			importPath: PgxImportPath,
			methodName: "Connect",
			wantVar:    "",
		},
		{
			name: "not an assignment statement",
			stmt: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.Ident{Name: "Connect", Path: PgxImportPath},
				},
			},
			importPath: PgxImportPath,
			methodName: "Connect",
			wantVar:    "",
		},
		{
			name: "wrong arg count is not detected",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{&dst.Ident{Name: "conn"}, &dst.Ident{Name: "err"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun:  &dst.Ident{Name: "Connect", Path: PgxImportPath},
						Args: []dst.Expr{&dst.Ident{Name: "ctx"}},
					},
				},
			},
			importPath: PgxImportPath,
			methodName: "Connect",
			wantVar:    "",
		},
		{
			name: "selector-form call is not detected",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{&dst.Ident{Name: "conn"}, &dst.Ident{Name: "err"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "pgx"},
							Sel: &dst.Ident{Name: "Connect"},
						},
						Args: []dst.Expr{
							&dst.Ident{Name: "ctx"},
							&dst.BasicLit{Value: `"postgres://localhost/mydb"`},
						},
					},
				},
			},
			importPath: PgxImportPath,
			methodName: "Connect",
			wantVar:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			gotVar, ctxExpr, connStrExpr := detectPgxCallPattern(tt.stmt, tt.importPath, tt.methodName)
			assert.Equal(t, tt.wantVar, gotVar)
			if tt.wantVar == "" {
				assert.Nil(t, ctxExpr)
				assert.Nil(t, connStrExpr)
			} else {
				assert.NotNil(t, ctxExpr)
				assert.NotNil(t, connStrExpr)
			}
		})
	}
}

func TestHasExistingPgxTracer(t *testing.T) {
	tracerAssignIdent := &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.SelectorExpr{X: dst.NewIdent("config"), Sel: dst.NewIdent("Tracer")},
		},
		Tok: token.ASSIGN,
		Rhs: []dst.Expr{
			&dst.CallExpr{Fun: &dst.Ident{Name: "NewTracer", Path: Nrpgx5ImportPath}},
		},
	}
	tracerAssignSelector := &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.SelectorExpr{X: dst.NewIdent("config"), Sel: dst.NewIdent("Tracer")},
		},
		Tok: token.ASSIGN,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{X: dst.NewIdent("nrpgx5"), Sel: dst.NewIdent("NewTracer")},
			},
		},
	}
	// pgx.NewTracer is a different package's NewTracer — must not match.
	foreignNewTracer := &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.SelectorExpr{X: dst.NewIdent("config"), Sel: dst.NewIdent("Tracer")},
		},
		Tok: token.ASSIGN,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{X: dst.NewIdent("pgx"), Sel: dst.NewIdent("NewTracer")},
			},
		},
	}
	unrelatedStmt := &dst.AssignStmt{
		Lhs: []dst.Expr{dst.NewIdent("x")},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{&dst.BasicLit{Value: `"hello"`}},
	}

	tests := []struct {
		name string
		body *dst.BlockStmt
		want bool
	}{
		{
			name: "nil body",
			body: nil,
			want: false,
		},
		{
			name: "empty body",
			body: &dst.BlockStmt{List: []dst.Stmt{}},
			want: false,
		},
		{
			name: "body without tracer",
			body: &dst.BlockStmt{List: []dst.Stmt{unrelatedStmt}},
			want: false,
		},
		{
			name: "body with tracer (Ident form)",
			body: &dst.BlockStmt{List: []dst.Stmt{tracerAssignIdent}},
			want: true,
		},
		{
			name: "body with tracer (SelectorExpr form)",
			body: &dst.BlockStmt{List: []dst.Stmt{tracerAssignSelector}},
			want: true,
		},
		{
			name: "tracer after other statements",
			body: &dst.BlockStmt{List: []dst.Stmt{unrelatedStmt, tracerAssignIdent}},
			want: true,
		},
		{
			name: "foreign package's NewTracer is not a match",
			body: &dst.BlockStmt{List: []dst.Stmt{foreignNewTracer}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasExistingPgxTracer(tt.body)
			assert.Equal(t, tt.want, got)
		})
	}
}
