package parser

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

const (
	sqlImportPath = "database/sql"
)

// detectSQLExecutionCall checks if a statement contains a SQL query operation using the given DB variable.
// Returns the method name (QueryRow, Query, or Exec) if found, otherwise returns an empty string.
//
// Example: row := db.QueryRow("SELECT count(*) from tables")
//
//	^^
func detectSQLExecutionCall(stmt dst.Stmt, dbName string) string {
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

	// Verify the method is called on the correct DB variable
	dbIdent, ok := selExpr.X.(*dst.Ident)
	if !ok || dbIdent.Name != dbName {
		return ""
	}

	// Check if it's a supported SQL operation method
	methodName := selExpr.Sel.Name
	switch methodName {
	case "QueryRow", "Query", "Exec":
		return methodName
	default:
		return ""
	}
}

// replaceSQLMethodWithContext replaces a SQL method with its context-aware version
// and prepends the context as the first argument.
//
// Transformations:
//   - QueryRow -> QueryRowContext
//   - Query    -> QueryContext
//   - Exec     -> ExecContext
func replaceSQLMethodWithContext(stmt dst.Stmt, ctxName string) {
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

	// Replace method name with Context version
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

	// Prepend context as first argument
	call.Args = append([]dst.Expr{&dst.Ident{Name: ctxName}}, call.Args...)
}

// detectSQLOpenCall returns the variable name from a sql.Open() call.
// Returns an empty string if the statement is not a sql.Open() call.
//
// Example: db, err := sql.Open("nrmysql", "root@/information_schema")
//
//	^^
func detectSQLOpenCall(stmt dst.Stmt) string {
	assignStmt, ok := stmt.(*dst.AssignStmt)
	if !ok || len(assignStmt.Rhs) != 1 || len(assignStmt.Lhs) == 0 {
		return ""
	}

	call, ok := assignStmt.Rhs[0].(*dst.CallExpr)
	if !ok {
		return ""
	}

	ident, ok := call.Fun.(*dst.Ident)
	if !ok || ident.Name != "Open" || ident.Path != sqlImportPath {
		return ""
	}

	// Extract the DB variable name from the left-hand side
	if dbIdent, ok := assignStmt.Lhs[0].(*dst.Ident); ok {
		return dbIdent.Name
	}

	return ""
}

// findLastUsageOfExecutionResult scans statements starting from startIndex to find the last
// usage of the given variable name. Returns the index of the last usage, or startIndex
// if the variable is never used after that point.
func findLastUsageOfExecutionResult(stmts []dst.Stmt, varName string, startIndex int) int {
	lastUsageIndex := startIndex

	for i := startIndex + 1; i < len(stmts); i++ {
		found := false

		dstutil.Apply(stmts[i], func(c *dstutil.Cursor) bool {
			if ident, ok := c.Node().(*dst.Ident); ok && ident.Name == varName {
				found = true
				return false // Stop traversal once found
			}
			return true
		}, nil)

		if found {
			lastUsageIndex = i
		}
	}

	return lastUsageIndex
}

// InstrumentSQLHandler instruments SQL database operations in the main function by:
// 1. Detecting sql.Open() calls to find the DB variable
// 2. Finding SQL execution calls (QueryRow, Query, Exec)
// 3. Creating a New Relic transaction around the SQL operation
// 4. Converting SQL methods to their context-aware versions
// 5. Inserting the transaction context before the SQL call
// 6. Ending the transaction after the result is consumed
func InstrumentSQLHandler(manager *InstrumentationManager, c *dstutil.Cursor) {
	funcDecl, ok := c.Node().(*dst.FuncDecl)
	if !ok || funcDecl.Name.Name != "main" {
		return
	}

	// Track state while scanning function body
	var (
		sqlDB             string // DB variable name (e.g., "db")
		sqlExecutionIndex int    = -1
		sqlResultVar      string // Result variable name (e.g., "row")
		sqlMethodName     string // SQL method name (e.g., "QueryRow")
	)

	// Scan function body to find SQL operations
	for i, stmt := range funcDecl.Body.List {
		// Detect sql.Open() to find the DB variable
		if dbName := detectSQLOpenCall(stmt); dbName != "" {
			sqlDB = dbName
			continue
		}

		// Once we have a DB variable, look for SQL execution calls
		if sqlDB != "" {
			if methodName := detectSQLExecutionCall(stmt, sqlDB); methodName != "" {
				sqlExecutionIndex = i
				sqlMethodName = methodName

				// Extract the result variable name from the assignment
				if assignStmt, ok := stmt.(*dst.AssignStmt); ok && len(assignStmt.Lhs) > 0 {
					if ident, ok := assignStmt.Lhs[0].(*dst.Ident); ok {
						sqlResultVar = ident.Name
					}
				}
				break // Only instrument the first SQL operation
			}
		}
	}

	// Exit if no SQL execution was found
	if sqlExecutionIndex == -1 {
		return
	}

	// Find where the SQL result variable is last used to determine where to end the transaction
	lastUsageIndex := sqlExecutionIndex
	if sqlResultVar != "" {
		lastUsageIndex = findLastUsageOfExecutionResult(funcDecl.Body.List, sqlResultVar, sqlExecutionIndex)
	}

	// Generate instrumentation code
	txnName := codegen.DefaultTransactionVariable
	ctxName := "ctx"

	txnStart := codegen.CreateSQLTransaction(manager.agentVariableName, txnName, sqlMethodName)
	ctxAssignment := codegen.CreateContextWithTransaction(ctxName, txnName)
	txnEnd := codegen.CreateTransactionEnd(txnName)

	// Transform the SQL method to use context (e.g., QueryRow -> QueryRowContext)
	replaceSQLMethodWithContext(funcDecl.Body.List[sqlExecutionIndex], ctxName)

	// Insert transaction end after the last usage of the result variable
	funcDecl.Body.List = append(
		funcDecl.Body.List[:lastUsageIndex+1],
		append([]dst.Stmt{txnEnd}, funcDecl.Body.List[lastUsageIndex+1:]...)...,
	)

	// Insert transaction start and context creation before the SQL execution
	funcDecl.Body.List = append(
		funcDecl.Body.List[:sqlExecutionIndex],
		append([]dst.Stmt{txnStart, ctxAssignment}, funcDecl.Body.List[sqlExecutionIndex:]...)...,
	)

	// Add required imports
	manager.addImport(codegen.NewRelicAgentImportPath)
	manager.addImport("context")

	// Wrap the main function with New Relic agent initialization
	tc := tracestate.FunctionBody(txnName)
	tc.WrapWithTransaction(c, "main", txnName)
}
