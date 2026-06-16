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
	state      driverState
	index      int             // index into body.List of the matching statement (-1 if state == driverNone)
	dbVar      string          // LHS variable name; "_" if blank identifier
	driverArg  *dst.BasicLit   // pointer into the AST so callers can mutate the driver name in place
}

// scanForPostgresOpen walks body once and returns the first sql.Open call whose driver string is
// either "postgres" (instrumentable) or "nrpq" (already done). Combines what used to be two
// separate scans (HasExistingNrpqDriver + findPQOpenStatement) so we don't loop the body twice.
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
func swapLibpqImportInPackage(manager *parser.InstrumentationManager) {
	pkg := manager.GetDecoratorPackage()
	if pkg == nil {
		return
	}

	for _, file := range pkg.Syntax {
		for _, imp := range file.Imports {
			if imp.Path.Value == `"github.com/lib/pq"` && imp.Name != nil && imp.Name.Name == "_" {
				imp.Path.Value = `"github.com/newrelic/go-agent/v3/integrations/nrpq"`
				manager.AddImport(NrpqImportPath)
				return
			}
		}
	}
}

// findSQLExecutionStatement scans stmts starting at startIndex for the first SQL execution call
// on dbName. Returns the index, method name, and result variable, or -1/"" if not found. A result
// variable of "_" (blank identifier) is reported as an empty string so callers can treat it as
// "no usable handle" without a separate check.
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
// wrapping the execution with a New Relic transaction. Driver swap runs in any function (named
// or literal); the transaction wrap only runs in main, where the agent variable is in scope.
//
// We can't (yet) propagate the transaction across function boundaries here — that would require
// using parser.TraceFunction / tracestate to thread the transaction through callers. Until that's
// wired in, an `initDB()` helper still gets the driver swap, and queries inside main still get
// wrapped, but queries reached only via a helper function won't.
func InstrumentPQHandler(manager *parser.InstrumentationManager, c *dstutil.Cursor) {
	var (
		body    *dst.BlockStmt
		isMain  bool
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
	switch scan.state {
	case driverNone:
		return
	case driverAlreadyNRPQ:
		// Already instrumented — nothing to do for this body. We don't swap the lib/pq import
		// here because some other body may still need it, and the import-swap is idempotent
		// (it's a no-op once already swapped).
		return
	case driverPostgres:
		// fall through
	}

	// Driver swap is safe everywhere: it's a single-arg mutation with no scope dependencies.
	scan.driverArg.Value = nrpqDriver
	swapLibpqImportInPackage(manager)

	// Transaction wrap requires the agent variable to be reachable. Without cross-function
	// transaction propagation, that limits us to main. A blank-identifier DB var (`_, err := …`)
	// also can't be queried, so there's nothing to wrap.
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
	ctxName := "ctx"

	// FindLastUsageOfExecutionResult tolerates an empty resultVar (returns startIndex unchanged),
	// so we can pass it through without an extra guard.
	lastUsage := sqlhelpers.FindLastUsageOfExecutionResult(body.List, resultVar, execIdx)

	sqlhelpers.ReplaceSQLMethodWithContext(body.List[execIdx], ctxName)

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
		createContextWithTransaction(ctxName, txnName),
	}, body.List[execIdx:])

	manager.AddImport(codegen.NewRelicAgentImportPath)
}
