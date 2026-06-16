package nrmysql

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/sqlhelpers"
	"github.com/newrelic/go-easy-instrumentation/parser"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

const (
	mysqlDriverName   = `"mysql"`
	nrmysqlDriverName = `"nrmysql"`
)

// InstrumentSQLHandler instruments SQL database operations in the main function by:
// 1. Detecting sql.Open() calls to find the DB variable
// 2. Finding SQL execution calls (QueryRow, Query, Exec)
// 3. Creating a New Relic transaction around the SQL operation
// 4. Converting SQL methods to their context-aware versions
// 5. Inserting the transaction context before the SQL call
// 6. Ending the transaction after the result is consumed
func InstrumentSQLHandler(manager *parser.InstrumentationManager, c *dstutil.Cursor) {
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
		// Detect sql.Open() with a MySQL-compatible driver to find the DB variable
		if dbName, driverArg := sqlhelpers.DetectSQLOpen(stmt); dbName != "" && driverArg != nil && isMySQLDriverArg(driverArg) {
			sqlDB = dbName
			continue
		}

		// Once we have a DB variable, look for SQL execution calls
		if sqlDB != "" {
			if methodName := sqlhelpers.DetectSQLExecutionCall(stmt, sqlDB); methodName != "" {
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
	lastUsageIndex := sqlhelpers.FindLastUsageOfExecutionResult(funcDecl.Body.List, sqlResultVar, sqlExecutionIndex)

	// Generate instrumentation code
	txnName := codegen.DefaultTransactionVariable
	ctxName := "ctx"

	txnStart := CreateSQLTransaction(manager.AgentVariableName(), txnName, sqlMethodName)
	ctxAssignment := CreateContextWithTransaction(ctxName, txnName)
	txnEnd := CreateTransactionEnd(txnName)

	// Transform the SQL method to use context (e.g., QueryRow -> QueryRowContext)
	sqlhelpers.ReplaceSQLMethodWithContext(funcDecl.Body.List[sqlExecutionIndex], ctxName)

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
	manager.AddImport(codegen.NewRelicAgentImportPath)

	// Wrap the main function with New Relic agent initialization
	tc := tracestate.FunctionBody(txnName)
	tc.WrapWithTransaction(c, "main", txnName)
}

// isMySQLDriverArg returns true if the driver-name BasicLit identifies a MySQL-compatible driver.
func isMySQLDriverArg(driverArg *dst.BasicLit) bool {
	return driverArg.Value == mysqlDriverName || driverArg.Value == nrmysqlDriverName
}
