package parser

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

const (
	gochiImportPath = "github.com/go-chi/chi"
)

// Return the variable name of the Chi router object.
// Ex:
//
//	router := chi.NewRouter()
//
// would return "router"
func getChiRouterName(stmt dst.Stmt) string {
	// Verify we're dealing with an assignment operation
	v, ok := stmt.(*dst.AssignStmt)
	if !ok || len(v.Rhs) != 1 {
		return ""
	}

	if v.Lhs == nil {
		return ""
	}

	// Verify the Rhs of the assignment is a Call Expression
	call, ok := v.Rhs[0].(*dst.CallExpr)
	if !ok {
		return ""
	}

	// Verify the name and path of the function being called
	ident, ok := call.Fun.(*dst.Ident)
	if !ok {
		return ""
	}

	// Reject calls that are not to the `NewRouter` Fn. Verify Chi relationship with the import path.
	if ident.Name != "NewRouter" || ident.Path != gochiImportPath {
		return ""
	}

	return v.Lhs[0].(*dst.Ident).Name
}

// Inject New Relic Middleware to the Chi router via the `Use` directive
func InstrumentChiMiddleware(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool {
	routerName := getChiRouterName(stmt)
	if routerName == "" {
		return false
	}

	// Append at the current stmt location
	middleware, goGet := codegen.NrChiMiddleware(routerName, tracing.AgentVariable())
	c.InsertAfter(middleware)
	manager.addImport(goGet)
	return true
}
