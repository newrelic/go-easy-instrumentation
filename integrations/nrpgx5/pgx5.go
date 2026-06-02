package nrpgx5

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/parser"
)

const (
	PgxImportPath     = "github.com/jackc/pgx/v5"
	PgxPoolImportPath = "github.com/jackc/pgx/v5/pgxpool"
	Nrpgx5ImportPath  = "github.com/newrelic/go-agent/v3/integrations/nrpgx5"
)

// InstrumentPgxHandler instruments pgx/v5 connections by injecting an nrpgx5 tracer into the
// connection config. It handles both direct connections (pgx.Connect) and connection pools
// (pgxpool.New), transforming each into a three-statement ParseConfig + Tracer + Connect sequence.
func InstrumentPgxHandler(manager *parser.InstrumentationManager, c *dstutil.Cursor) {
	funcDecl, ok := c.Node().(*dst.FuncDecl)
	if !ok {
		return
	}

	for i, stmt := range funcDecl.Body.List {
		replacement := buildPgxReplacement(stmt)
		if replacement == nil {
			continue
		}
		funcDecl.Body.List = replaceStatement(funcDecl.Body.List, i, replacement)
		manager.AddImport(Nrpgx5ImportPath)
		return
	}
}

// buildPgxReplacement detects a pgx.Connect or pgxpool.New call and returns the three replacement
// statements that inject the nrpgx5 tracer. Returns nil if the statement is not a recognized call.
func buildPgxReplacement(stmt dst.Stmt) []dst.Stmt {
	if connVar, ctxExpr, connStrExpr := DetectPgxConnectCall(stmt); connVar != "" {
		return []dst.Stmt{
			CreatePgxParseConfig("config", connStrExpr),
			CreateTracerAssignment("config"),
			CreatePgxConnectConfig(connVar, ctxExpr, "config"),
		}
	}

	if poolVar, ctxExpr, connStrExpr := DetectPgxPoolNewCall(stmt); poolVar != "" {
		return []dst.Stmt{
			CreatePgxPoolParseConfig("config", connStrExpr),
			CreatePoolTracerAssignment("config"),
			CreatePgxPoolNewWithConfig(poolVar, ctxExpr, "config"),
		}
	}

	return nil
}

// replaceStatement replaces stmts[i] with one or more replacement statements.
func replaceStatement(stmts []dst.Stmt, i int, replacement []dst.Stmt) []dst.Stmt {
	return append(stmts[:i], append(replacement, stmts[i+1:]...)...)
}

// DetectPgxConnectCall checks if a statement is a pgx.Connect(ctx, connStr) call.
// Returns the connection variable name and the ctx and connStr expressions if found.
//
// Example: conn, err := pgx.Connect(ctx, "postgres://...")
//
//	^^^^
func DetectPgxConnectCall(stmt dst.Stmt) (connVar string, ctxExpr dst.Expr, connStrExpr dst.Expr) {
	return detectPgxCallPattern(stmt, PgxImportPath, "Connect")
}

// DetectPgxPoolNewCall checks if a statement is a pgxpool.New(ctx, connStr) call.
// Returns the pool variable name and the ctx and connStr expressions if found.
//
// Example: pool, err := pgxpool.New(ctx, "postgres://...")
//
//	^^^^
func DetectPgxPoolNewCall(stmt dst.Stmt) (poolVar string, ctxExpr dst.Expr, connStrExpr dst.Expr) {
	return detectPgxCallPattern(stmt, PgxPoolImportPath, "New")
}

// detectPgxCallPattern is the shared detection logic for pgx and pgxpool connection calls.
// It matches: varName, err := pkg.Method(ctx, connStr)
// where pkg is identified by importPath (DST sets ident.Path on package-qualified function calls).
func detectPgxCallPattern(stmt dst.Stmt, importPath, methodName string) (varName string, ctxExpr dst.Expr, connStrExpr dst.Expr) {
	assignStmt, ok := stmt.(*dst.AssignStmt)
	if !ok || len(assignStmt.Rhs) != 1 || len(assignStmt.Lhs) == 0 {
		return "", nil, nil
	}

	call, ok := assignStmt.Rhs[0].(*dst.CallExpr)
	if !ok || len(call.Args) != 2 {
		return "", nil, nil
	}

	// DST represents package-qualified calls as *dst.Ident with Path set to the import path,
	// not as a SelectorExpr — this is DST's special handling for imported package functions.
	ident, ok := call.Fun.(*dst.Ident)
	if !ok || ident.Name != methodName || ident.Path != importPath {
		return "", nil, nil
	}

	lhsIdent, ok := assignStmt.Lhs[0].(*dst.Ident)
	if !ok {
		return "", nil, nil
	}

	return lhsIdent.Name, call.Args[0], call.Args[1]
}
