package nrpgx5

import (
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

func TestCreateParseConfig(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
	}{
		{name: "pgx", importPath: PgxImportPath},
		{name: "pgxpool", importPath: PgxPoolImportPath},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connStr := &dst.BasicLit{Kind: token.STRING, Value: `"postgres://localhost/mydb"`}
			got := CreateParseConfig(connStr, tt.importPath)

			assert.NotNil(t, got)
			assert.Equal(t, token.DEFINE, got.Tok)
			assert.Len(t, got.Lhs, 2)

			configIdent, ok := got.Lhs[0].(*dst.Ident)
			assert.True(t, ok)
			assert.Equal(t, configVar, configIdent.Name)

			errIdent, ok := got.Lhs[1].(*dst.Ident)
			assert.True(t, ok)
			assert.Equal(t, "err", errIdent.Name)

			call, ok := got.Rhs[0].(*dst.CallExpr)
			assert.True(t, ok)
			assert.Len(t, call.Args, 1)

			funIdent, ok := call.Fun.(*dst.Ident)
			assert.True(t, ok)
			assert.Equal(t, "ParseConfig", funIdent.Name)
			assert.Equal(t, tt.importPath, funIdent.Path)

			// Connection string is cloned, not aliased — original must be untouched.
			arg, ok := call.Args[0].(*dst.BasicLit)
			assert.True(t, ok)
			assert.Equal(t, connStr.Value, arg.Value)
			assert.NotSame(t, connStr, arg)
		})
	}
}

func TestCreateTracerAssignment(t *testing.T) {
	tests := []struct {
		name        string
		receiver    dst.Expr
		assertOnLhs func(t *testing.T, lhs *dst.SelectorExpr)
	}{
		{
			name:     "direct config receiver (pgx)",
			receiver: dst.NewIdent(configVar),
			assertOnLhs: func(t *testing.T, lhs *dst.SelectorExpr) {
				assert.Equal(t, "Tracer", lhs.Sel.Name)
				ident, ok := lhs.X.(*dst.Ident)
				assert.True(t, ok)
				assert.Equal(t, configVar, ident.Name)
			},
		},
		{
			name: "nested config.ConnConfig receiver (pgxpool)",
			receiver: &dst.SelectorExpr{
				X:   dst.NewIdent(configVar),
				Sel: dst.NewIdent("ConnConfig"),
			},
			assertOnLhs: func(t *testing.T, lhs *dst.SelectorExpr) {
				assert.Equal(t, "Tracer", lhs.Sel.Name)
				inner, ok := lhs.X.(*dst.SelectorExpr)
				assert.True(t, ok)
				assert.Equal(t, "ConnConfig", inner.Sel.Name)
				ident, ok := inner.X.(*dst.Ident)
				assert.True(t, ok)
				assert.Equal(t, configVar, ident.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateTracerAssignment(tt.receiver)

			assert.NotNil(t, got)
			assert.Equal(t, token.ASSIGN, got.Tok)
			assert.Len(t, got.Lhs, 1)

			lhsSel, ok := got.Lhs[0].(*dst.SelectorExpr)
			assert.True(t, ok)
			tt.assertOnLhs(t, lhsSel)

			call, ok := got.Rhs[0].(*dst.CallExpr)
			assert.True(t, ok)

			funIdent, ok := call.Fun.(*dst.Ident)
			assert.True(t, ok)
			assert.Equal(t, "NewTracer", funIdent.Name)
			assert.Equal(t, Nrpgx5ImportPath, funIdent.Path)
		})
	}
}

func TestCreateConnectWithConfig(t *testing.T) {
	tests := []struct {
		name       string
		varName    string
		fun        dst.Expr
		wantMethod string
		wantPath   string
	}{
		{
			name:       "pgx.ConnectConfig",
			varName:    "conn",
			fun:        &dst.Ident{Name: "ConnectConfig", Path: PgxImportPath},
			wantMethod: "ConnectConfig",
			wantPath:   PgxImportPath,
		},
		{
			name:       "pgxpool.NewWithConfig",
			varName:    "pool",
			fun:        &dst.Ident{Name: "NewWithConfig", Path: PgxPoolImportPath},
			wantMethod: "NewWithConfig",
			wantPath:   PgxPoolImportPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxExpr := &dst.Ident{Name: "ctx"}
			got := CreateConnectWithConfig(tt.varName, ctxExpr, tt.fun)

			assert.NotNil(t, got)
			assert.Equal(t, token.DEFINE, got.Tok)
			assert.Len(t, got.Lhs, 2)

			lhsIdent, ok := got.Lhs[0].(*dst.Ident)
			assert.True(t, ok)
			assert.Equal(t, tt.varName, lhsIdent.Name)

			errIdent, ok := got.Lhs[1].(*dst.Ident)
			assert.True(t, ok)
			assert.Equal(t, "err", errIdent.Name)

			call, ok := got.Rhs[0].(*dst.CallExpr)
			assert.True(t, ok)

			funIdent, ok := call.Fun.(*dst.Ident)
			assert.True(t, ok)
			assert.Equal(t, tt.wantMethod, funIdent.Name)
			assert.Equal(t, tt.wantPath, funIdent.Path)

			assert.Len(t, call.Args, 2)
			configIdent, ok := call.Args[1].(*dst.Ident)
			assert.True(t, ok)
			assert.Equal(t, configVar, configIdent.Name)
		})
	}
}
