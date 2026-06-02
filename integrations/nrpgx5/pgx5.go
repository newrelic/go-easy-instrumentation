package nrpgx5

import (
	"slices"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/parser"
)

// InstrumentPgxHandler instruments pgx/v5 connections by injecting an nrpgx5 tracer into the
// connection config. It handles both direct connections (pgx.Connect) and connection pools
// (pgxpool.New), transforming each into a three-statement ParseConfig + Tracer + Connect sequence.
// It handles both named functions and function literals.
func InstrumentPgxHandler(manager *parser.InstrumentationManager, c *dstutil.Cursor) {
	var body *dst.BlockStmt
	switch node := c.Node().(type) {
	case *dst.FuncDecl:
		body = node.Body
	case *dst.FuncLit:
		body = node.Body
	default:
		return
	}

	if HasExistingPgxTracer(body) {
		comment.Debug(manager.GetDecoratorPackage(), body, "pgx tracer already configured, skipping")
		return
	}

	for i, stmt := range body.List {
		replacement := buildPgxReplacement(stmt)
		if replacement == nil {
			continue
		}
		body.List = slices.Concat(body.List[:i], replacement, body.List[i+1:])
		manager.AddImport(Nrpgx5ImportPath)
		return
	}
}

// HasExistingPgxTracer reports whether body already contains an nrpgx5.NewTracer() assignment,
// indicating the pgx connection has already been instrumented.
func HasExistingPgxTracer(body *dst.BlockStmt) bool {
	if body == nil {
		return false
	}
	for _, stmt := range body.List {
		assign, ok := stmt.(*dst.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			continue
		}
		call, ok := assign.Rhs[0].(*dst.CallExpr)
		if !ok {
			continue
		}
		switch fun := call.Fun.(type) {
		case *dst.Ident:
			// DST represents package-qualified calls as *dst.Ident with Path set to the import path.
			if fun.Name == "NewTracer" && fun.Path == Nrpgx5ImportPath {
				return true
			}
		case *dst.SelectorExpr:
			// Without full type info, DST uses SelectorExpr instead of Ident with Path.
			_, ok := fun.X.(*dst.Ident)
			if ok && fun.Sel.Name == "NewTracer" {
				return true
			}
		}
	}
	return false
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
