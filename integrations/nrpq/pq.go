package nrpq

import (
	"fmt"
	"slices"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/sqlhelpers"
	"github.com/newrelic/go-easy-instrumentation/parser"
)

const (
	// LibpqImportPath is the import path for the lib/pq PostgreSQL driver.
	LibpqImportPath = "github.com/lib/pq"
	// NrpqImportPath is the import path for the New Relic nrpq PostgreSQL driver wrapper.
	NrpqImportPath = "github.com/newrelic/go-agent/v3/integrations/nrpq"

	postgresDriver = `"postgres"`
	nrpqDriver     = `"nrpq"`
)

// driverState classifies what we found at a `sql.Open(...)` site.
type driverState int

const (
	driverNone        driverState = iota // not a sql.Open call we recognize
	driverPostgres                       // sql.Open("postgres", ...) — needs swapping
	driverAlreadyNRPQ                    // sql.Open("nrpq", ...) — already instrumented, skip
)

// pqOpenScan is the result of a single pass over a function body looking for the postgres open.
type pqOpenScan struct {
	state     driverState
	index     int           // index into body.List of the matching statement (-1 if state == driverNone)
	dbVar     string        // LHS variable name; "_" if blank identifier
	driverArg *dst.BasicLit // pointer into the AST so callers can mutate the driver name in place
}

// scanForPostgresOpen walks body and returns the first sql.Open call whose driver string is
// either "postgres" (instrumentable) or "nrpq" (already instrumented). The result distinguishes
// the two states so callers can early-return on either.
func scanForPostgresOpen(body *dst.BlockStmt) pqOpenScan {
	if body == nil {
		return pqOpenScan{state: driverNone, index: -1}
	}
	for i, stmt := range body.List {
		dbVar, driverArg := sqlhelpers.DetectSQLOpen(stmt)
		if driverArg == nil {
			continue
		}
		switch driverArg.Value {
		case nrpqDriver:
			return pqOpenScan{state: driverAlreadyNRPQ, index: i, dbVar: dbVar, driverArg: driverArg}
		case postgresDriver:
			return pqOpenScan{state: driverPostgres, index: i, dbVar: dbVar, driverArg: driverArg}
		}
	}
	return pqOpenScan{state: driverNone, index: -1}
}

// swapLibpqImportInPackage finds the blank lib/pq import across all files in the current package
// and replaces it with the New Relic nrpq driver wrapper.
//
// _ "github.com/lib/pq" -> _ "github.com/newrelic/go-agent/v3/integrations/nrpq"
// TO-DO: In the future, we may want to make this a shared utility in the manager class as we expand our integration library
func swapLibpqImportInPackage(manager *parser.InstrumentationManager) {
	pkg := manager.GetDecoratorPackage()
	if pkg == nil {
		return
	}

	for _, file := range pkg.Syntax {
		for _, imp := range file.Imports {
			if imp.Path.Value == `"`+LibpqImportPath+`"` && imp.Name != nil && imp.Name.Name == "_" {
				imp.Path.Value = `"` + NrpqImportPath + `"`
				manager.AddImport(NrpqImportPath)
				break
			}
		}
	}
}

// findSQLExecutionStatement scans stmts starting at startIndex for the first SQL execution call
// on dbName. Returns the index, method name, and result variable, or -1/"" if not found. A
// blank-identifier LHS (`_, err := …`) is reported with an empty resultVar.
func findSQLExecutionStatement(stmts []dst.Stmt, dbName string, startIndex int) (index int, method string, resultVar string) {
	for i := startIndex; i < len(stmts); i++ {
		m := sqlhelpers.DetectSQLExecutionCall(stmts[i], dbName)
		if m == "" {
			continue
		}
		if assign, ok := stmts[i].(*dst.AssignStmt); ok && len(assign.Lhs) > 0 {
			if ident, ok := assign.Lhs[0].(*dst.Ident); ok && ident.Name != "_" {
				resultVar = ident.Name
			}
		}
		return i, m, resultVar
	}
	return -1, "", ""
}

// InstrumentPQHandler instruments PostgreSQL database operations by swapping the driver name to
// "nrpq" and, when both the open and a SQL execution call live in the same function body, also
// wrapping the execution with a New Relic transaction. The driver swap runs in any function
// (named or literal); the transaction wrap only runs in main, where the agent variable is in
// scope.
func InstrumentPQHandler(manager *parser.InstrumentationManager, c *dstutil.Cursor) {
	var (
		body   *dst.BlockStmt
		isMain bool
	)
	switch node := c.Node().(type) {
	case *dst.FuncDecl:
		body = node.Body
		isMain = node.Name.Name == "main"
	case *dst.FuncLit:
		body = node.Body
	default:
		return
	}
	if body == nil {
		return
	}

	scan := scanForPostgresOpen(body)
	if scan.state != driverPostgres {
		return
	}

	scan.driverArg.Value = nrpqDriver
	swapLibpqImportInPackage(manager)

	// Transaction wrap only runs in main (agent variable is in scope) and requires a usable
	// DB handle (a blank-identifier LHS can't be referenced by a later execution call).
	if !isMain || scan.dbVar == "" || scan.dbVar == "_" {
		return
	}
	wrapWithTransaction(manager, body, scan.index, scan.dbVar)
}

// wrapWithTransaction inserts a New Relic transaction around the first SQL execution call on
// dbName that follows openIdx. No-op if no execution call is found.
func wrapWithTransaction(manager *parser.InstrumentationManager, body *dst.BlockStmt, openIdx int, dbName string) {
	execIdx, methodName, resultVar := findSQLExecutionStatement(body.List, dbName, openIdx+1)
	if execIdx == -1 {
		return
	}

	txnName := codegen.DefaultTransactionVariable

	lastUsage := sqlhelpers.FindLastUsageOfExecutionResult(body.List, resultVar, execIdx)

	sqlhelpers.ReplaceSQLMethodWithContext(body.List[execIdx], codegen.DefaultContextParameter)

	// Insert nrTxn.End() immediately after the last usage of the result variable:
	//   row.Scan(...)   <- lastUsage
	//   nrTxn.End()    <- inserted here
	body.List = slices.Concat(body.List[:lastUsage+1], []dst.Stmt{codegen.EndTransaction(txnName)}, body.List[lastUsage+1:])

	// Insert transaction start and context before the (now context-aware) SQL execution:
	//   nrTxn := NewRelicAgent.StartTransaction("postgres/QueryRow")   <- inserted
	//   ctx := newrelic.NewContext(context.Background(), nrTxn)        <- inserted
	//   row := db.QueryRowContext(ctx, ...)                            <- execIdx
	body.List = slices.Concat(body.List[:execIdx], []dst.Stmt{
		codegen.StartTransaction(manager.AgentVariableName(), txnName, fmt.Sprintf("postgres/%s", methodName), false),
		createContextWithTransaction(txnName),
	}, body.List[execIdx:])

	manager.AddImport(codegen.NewRelicAgentImportPath)
}
