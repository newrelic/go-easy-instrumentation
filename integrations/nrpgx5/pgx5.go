package nrpgx5

import (
	"slices"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/parser"
)

// nrpgx5PackageName is the conventional package alias for the nrpgx5 integration.
// Used as a fallback when DST has lost type info and represents the call as a SelectorExpr.
const nrpgx5PackageName = "nrpgx5"

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
			x, ok := fun.X.(*dst.Ident)
			if ok && x.Name == nrpgx5PackageName && fun.Sel.Name == "NewTracer" {
				return true
			}
		}
	}
	return false
}

// buildPgxReplacement detects a pgx.Connect or pgxpool.New call and returns the three replacement
// statements that inject the nrpgx5 tracer. Returns nil if the statement is not a recognized call.
func buildPgxReplacement(stmt dst.Stmt) []dst.Stmt {
	if connVar, ctxExpr, connStrExpr := detectPgxCallPattern(stmt, PgxImportPath, "Connect"); connVar != "" {
		return []dst.Stmt{
			CreateParseConfig(connStrExpr, PgxImportPath),
			CreateTracerAssignment(dst.NewIdent(configVar)),
			CreateConnectWithConfig(connVar, ctxExpr, &dst.Ident{Name: "ConnectConfig", Path: PgxImportPath}),
		}
	}

	if poolVar, ctxExpr, connStrExpr := detectPgxCallPattern(stmt, PgxPoolImportPath, "New"); poolVar != "" {
		return []dst.Stmt{
			CreateParseConfig(connStrExpr, PgxPoolImportPath),
			CreateTracerAssignment(&dst.SelectorExpr{
				X:   dst.NewIdent(configVar),
				Sel: dst.NewIdent("ConnConfig"),
			}),
			CreateConnectWithConfig(poolVar, ctxExpr, &dst.Ident{Name: "NewWithConfig", Path: PgxPoolImportPath}),
		}
	}

	return nil
}

// detectPgxCallPattern is the shared detection logic for pgx and pgxpool connection calls.
// It matches: varName, err := pkg.Method(ctx, connStr)
// where pkg is identified by importPath (DST sets ident.Path on package-qualified function calls).
//
// Returns the bound variable name plus the ctx and connStr expressions, or zero values if the
// statement does not match.
func detectPgxCallPattern(stmt dst.Stmt, importPath, methodName string) (varName string, ctxExpr dst.Expr, connStrExpr dst.Expr) {
	// Require a single-call RHS bound to at least one LHS name. The pgx and pgxpool entry
	// points we're matching all have the shape `x, err := pkg.Method(...)`, so anything with
	// multiple RHS expressions or an empty LHS cannot be the call we want.
	assignStmt, ok := stmt.(*dst.AssignStmt)
	if !ok || len(assignStmt.Rhs) != 1 || len(assignStmt.Lhs) == 0 {
		return "", nil, nil
	}

	// Both pgx.Connect and pgxpool.New take exactly two arguments (ctx, connStr); reject any
	// arity mismatch up front so we don't index past the args slice below.
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
