package nrpgx5_test

import (
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/integrations/nrpgx5"
	"github.com/stretchr/testify/assert"
)

func TestCreatePgxParseConfig(t *testing.T) {
	connStr := &dst.BasicLit{Kind: token.STRING, Value: `"postgres://localhost/mydb"`}
	got := nrpgx5.CreatePgxParseConfig("config", connStr)

	assert.NotNil(t, got)
	assert.Equal(t, token.DEFINE, got.Tok)
	assert.Len(t, got.Lhs, 2)

	configIdent, ok := got.Lhs[0].(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "config", configIdent.Name)

	call, ok := got.Rhs[0].(*dst.CallExpr)
	assert.True(t, ok)

	funIdent, ok := call.Fun.(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "ParseConfig", funIdent.Name)
	assert.Equal(t, nrpgx5.PgxImportPath, funIdent.Path)
	assert.Len(t, call.Args, 1)
}

func TestCreatePgxPoolParseConfig(t *testing.T) {
	connStr := &dst.BasicLit{Kind: token.STRING, Value: `"postgres://localhost/mydb"`}
	got := nrpgx5.CreatePgxPoolParseConfig("config", connStr)

	assert.NotNil(t, got)
	assert.Equal(t, token.DEFINE, got.Tok)

	call, ok := got.Rhs[0].(*dst.CallExpr)
	assert.True(t, ok)

	funIdent, ok := call.Fun.(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "ParseConfig", funIdent.Name)
	assert.Equal(t, nrpgx5.PgxPoolImportPath, funIdent.Path)
}

func TestCreateTracerAssignment(t *testing.T) {
	got := nrpgx5.CreateTracerAssignment("config")

	assert.NotNil(t, got)
	assert.Equal(t, token.ASSIGN, got.Tok)
	assert.Len(t, got.Lhs, 1)

	lhsSel, ok := got.Lhs[0].(*dst.SelectorExpr)
	assert.True(t, ok)
	assert.Equal(t, "Tracer", lhsSel.Sel.Name)

	lhsX, ok := lhsSel.X.(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "config", lhsX.Name)

	call, ok := got.Rhs[0].(*dst.CallExpr)
	assert.True(t, ok)

	funIdent, ok := call.Fun.(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "NewTracer", funIdent.Name)
	assert.Equal(t, nrpgx5.Nrpgx5ImportPath, funIdent.Path)
}

func TestCreatePoolTracerAssignment(t *testing.T) {
	got := nrpgx5.CreatePoolTracerAssignment("config")

	assert.NotNil(t, got)
	assert.Equal(t, token.ASSIGN, got.Tok)

	// LHS should be config.ConnConfig.Tracer
	outerSel, ok := got.Lhs[0].(*dst.SelectorExpr)
	assert.True(t, ok)
	assert.Equal(t, "Tracer", outerSel.Sel.Name)

	innerSel, ok := outerSel.X.(*dst.SelectorExpr)
	assert.True(t, ok)
	assert.Equal(t, "ConnConfig", innerSel.Sel.Name)

	configIdent, ok := innerSel.X.(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "config", configIdent.Name)

	call, ok := got.Rhs[0].(*dst.CallExpr)
	assert.True(t, ok)

	funIdent, ok := call.Fun.(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "NewTracer", funIdent.Name)
	assert.Equal(t, nrpgx5.Nrpgx5ImportPath, funIdent.Path)
}

func TestCreatePgxConnectConfig(t *testing.T) {
	ctxExpr := &dst.Ident{Name: "ctx"}
	got := nrpgx5.CreatePgxConnectConfig("conn", ctxExpr, "config")

	assert.NotNil(t, got)
	assert.Equal(t, token.DEFINE, got.Tok)
	assert.Len(t, got.Lhs, 2)

	connIdent, ok := got.Lhs[0].(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "conn", connIdent.Name)

	call, ok := got.Rhs[0].(*dst.CallExpr)
	assert.True(t, ok)

	funIdent, ok := call.Fun.(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "ConnectConfig", funIdent.Name)
	assert.Equal(t, nrpgx5.PgxImportPath, funIdent.Path)

	assert.Len(t, call.Args, 2)
	configIdent, ok := call.Args[1].(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "config", configIdent.Name)
}

func TestCreatePgxPoolNewWithConfig(t *testing.T) {
	ctxExpr := &dst.Ident{Name: "ctx"}
	got := nrpgx5.CreatePgxPoolNewWithConfig("pool", ctxExpr, "config")

	assert.NotNil(t, got)
	assert.Equal(t, token.DEFINE, got.Tok)
	assert.Len(t, got.Lhs, 2)

	poolIdent, ok := got.Lhs[0].(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "pool", poolIdent.Name)

	call, ok := got.Rhs[0].(*dst.CallExpr)
	assert.True(t, ok)

	funIdent, ok := call.Fun.(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "NewWithConfig", funIdent.Name)
	assert.Equal(t, nrpgx5.PgxPoolImportPath, funIdent.Path)

	assert.Len(t, call.Args, 2)
	configIdent, ok := call.Args[1].(*dst.Ident)
	assert.True(t, ok)
	assert.Equal(t, "config", configIdent.Name)
}
