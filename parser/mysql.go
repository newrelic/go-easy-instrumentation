package parser

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

const (
	sqlImportPath = "database/sql"
)

// detectSQLQueryCall checks if a statement contains a SQL query operation using the given DB variable
// row := db.QueryRow("SELECT count(*) from tables")
//
//	^^
func detectSQLQueryCall(stmt dst.Stmt, dbName string) bool {
	assignStmt, ok := stmt.(*dst.AssignStmt)
	if !ok || len(assignStmt.Rhs) != 1 {
		return false
	}

	call, ok := assignStmt.Rhs[0].(*dst.CallExpr)
	if !ok {
		return false
	}

	// Check if it's a selector expression (e.g., db.QueryRow)
	selExpr, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the receiver is the DB variable
	dbIdent, ok := selExpr.X.(*dst.Ident)
	if !ok || dbIdent.Name != dbName {
		return false
	}

	// Check if it's a SQL operation method
	methodName := selExpr.Sel.Name
	if methodName == "QueryRow" || methodName == "Query" || methodName == "Exec" {
		fmt.Println("Found SQL operation:", dbName+"."+methodName)
		return true
	}

	return false
}

// sqlOperationCall returns the variable name of the sql QueryRow call so that new relic middleware can be appended
// db, err := sql.Open("nrmysql", "root@/information_schema")
// ^^
func detectSQLOpenCall(stmt dst.Stmt) string {
	v, ok := stmt.(*dst.AssignStmt)
	if !ok || len(v.Rhs) != 1 {
		return ""
	}
	if call, ok := v.Rhs[0].(*dst.CallExpr); ok {
		if ident, ok := call.Fun.(*dst.Ident); ok {
			if (ident.Name == "Open") && ident.Path == sqlImportPath {
				return v.Lhs[0].(*dst.Ident).Name
			}
		}
	}

	return ""
}

// Stateless Tracing Functions
// ////////////////////////////////////////////
// InstrumentSQLHandler will check to see if any sql.QueryRow calls are made within the main function
// NOTE: Should we be limiting this to main? Is it possible/widely accepted to initialize a logging library outside of main?
func InstrumentSQLHandler(manager *InstrumentationManager, c *dstutil.Cursor) {
	mainFunctionNode := c.Node()
	if decl, ok := mainFunctionNode.(*dst.FuncDecl); ok {
		if decl.Name.Name != "main" {
			return
		}

		var sqlDB string // Track the DB variable name across all statements

		// loop through all statements within the body of the main method
		for _, stmt := range decl.Body.List {
			// Check if this statement creates a new DB connection
			if dbName := detectSQLOpenCall(stmt); dbName != "" {
				sqlDB = dbName
				fmt.Println("Found SQL DB:", sqlDB)
			}

			// Check if this statement performs a SQL operation
			if sqlOp := detectSQLQueryCall(stmt, sqlDB); sqlOp {
				if sqlDB == "" {
					// This *should not* happen
					// TO:DO - This would be a good spot for error handling or logging
					fmt.Println("Warning: SQL operation", sqlOp, "found but no DB connection detected")
					continue
				}
				fmt.Println("Detected sqlOp", sqlOp, "on", sqlDB)
				return
			}
		}
	}
}
