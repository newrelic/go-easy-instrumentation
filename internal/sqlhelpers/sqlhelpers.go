// Package sqlhelpers contains driver-agnostic helpers shared across SQL integrations
// (nrmysql, nrpq, ...). Anything tied to a specific driver string belongs in the driver
// integration; anything that operates on the database/sql shape belongs here.
package sqlhelpers

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// SQLImportPath is the standard library import path for database/sql.
const SQLImportPath = "database/sql"

// DetectSQLOpen recognizes `varName, err := sql.Open(<driver>, <connStr>)` and returns the
// LHS variable name plus a pointer to the driver-name BasicLit. Callers can mutate the returned
// BasicLit's Value to swap drivers in place (e.g. `"postgres"` -> `"nrpq"`). If the statement
// is not a sql.Open call the returned varName is "" and driverArg is nil.
//
// A varName of "_" (blank identifier) is returned as-is — the open is still a real
// sql.Open call worth swapping the driver on, but downstream callers should treat "_"
// as "no usable handle" when looking for execution calls on the DB.
func DetectSQLOpen(stmt dst.Stmt) (varName string, driverArg *dst.BasicLit) {
	// We need a single-call RHS bound to at least one LHS name. sql.Open returns (*DB, error),
	// so the assignment shape is always `db, err := sql.Open(...)`.
	assign, ok := stmt.(*dst.AssignStmt)
	if !ok || len(assign.Rhs) != 1 || len(assign.Lhs) == 0 {
		return "", nil
	}

	// sql.Open takes exactly two arguments (driverName, dataSourceName); reject any other arity
	// up front so we can index Args[0] safely below.
	call, ok := assign.Rhs[0].(*dst.CallExpr)
	if !ok || len(call.Args) != 2 {
		return "", nil
	}

	// DST represents `sql.Open` as a *dst.Ident with Path set to "database/sql" rather than a
	// SelectorExpr — this is DST's special handling for imported package functions.
	ident, ok := call.Fun.(*dst.Ident)
	if !ok || ident.Name != "Open" || ident.Path != SQLImportPath {
		return "", nil
	}

	// The driver name is the first argument and must be a string literal for us to inspect it.
	lit, ok := call.Args[0].(*dst.BasicLit)
	if !ok {
		return "", nil
	}

	lhsIdent, ok := assign.Lhs[0].(*dst.Ident)
	if !ok {
		return "", nil
	}

	return lhsIdent.Name, lit
}

// DetectSQLExecutionCall reports whether stmt is a SQL execution call on dbName.
// Returns the method name ("QueryRow", "Query", or "Exec") if found, otherwise "".
//
// Example: row := db.QueryRow("SELECT count(*) from tables")
func DetectSQLExecutionCall(stmt dst.Stmt, dbName string) string {
	assignStmt, ok := stmt.(*dst.AssignStmt)
	if !ok || len(assignStmt.Rhs) != 1 {
		return ""
	}

	call, ok := assignStmt.Rhs[0].(*dst.CallExpr)
	if !ok {
		return ""
	}

	selExpr, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return ""
	}

	dbIdent, ok := selExpr.X.(*dst.Ident)
	if !ok || dbIdent.Name != dbName {
		return ""
	}

	switch selExpr.Sel.Name {
	case "QueryRow", "Query", "Exec":
		return selExpr.Sel.Name
	}
	return ""
}

// ReplaceSQLMethodWithContext rewrites a SQL execution call to its context-aware variant
// and prepends ctxName as the first argument:
//
//	row := db.QueryRow(...)  ->  row := db.QueryRowContext(ctx, ...)
//	rows, _ := db.Query(...) ->  rows, _ := db.QueryContext(ctx, ...)
//	res, _ := db.Exec(...)   ->  res, _ := db.ExecContext(ctx, ...)
//
// No-op if stmt is not a recognized SQL execution call.
func ReplaceSQLMethodWithContext(stmt dst.Stmt, ctxName string) {
	assignStmt, ok := stmt.(*dst.AssignStmt)
	if !ok || len(assignStmt.Rhs) != 1 {
		return
	}

	call, ok := assignStmt.Rhs[0].(*dst.CallExpr)
	if !ok {
		return
	}

	selExpr, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return
	}

	switch selExpr.Sel.Name {
	case "QueryRow":
		selExpr.Sel.Name = "QueryRowContext"
	case "Query":
		selExpr.Sel.Name = "QueryContext"
	case "Exec":
		selExpr.Sel.Name = "ExecContext"
	default:
		return
	}

	call.Args = append([]dst.Expr{&dst.Ident{Name: ctxName}}, call.Args...)
}

// FindLastUsageOfExecutionResult scans stmts after startIndex for the last statement that
// references varName, and returns that index. Returns startIndex if varName is never used after
// startIndex, or if varName is empty / the blank identifier (in which case there is no handle
// to follow).
func FindLastUsageOfExecutionResult(stmts []dst.Stmt, varName string, startIndex int) int {
	if varName == "" || varName == "_" {
		return startIndex
	}

	lastUsageIndex := startIndex
	for i := startIndex + 1; i < len(stmts); i++ {
		found := false
		dstutil.Apply(stmts[i], func(c *dstutil.Cursor) bool {
			if ident, ok := c.Node().(*dst.Ident); ok && ident.Name == varName {
				found = true
				return false
			}
			return true
		}, nil)
		if found {
			lastUsageIndex = i
		}
	}
	return lastUsageIndex
}
