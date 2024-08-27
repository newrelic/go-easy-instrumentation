package parser

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

const (
	// the default name for transaction variables
	defaultTxnName = "nrTxn"
	// the import path for the newrelic package
	newrelicAgentImport string = "github.com/newrelic/go-agent/v3/newrelic"
)

// StatefulTracingFunctions are functions that require knowledge of the current tracing state of the package to apply instrumentation.
// These functions are passed the current tracing state of the package, and return a boolean indicating whether the function was modified.
// If the function was modified, it is likely that a transaction is required.
// The tracingName parameter is used to identify the object containing a New Relic Transaction.
// These functions are invoked on every statement in the body of a function that is being traced by the TraceFunction function.
type StatefulTracingFunction func(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracingState) bool

// StatelessTracingFunction is a function that does not need to be aware of the current tracing state of the package to apply instrumentation.
// These functions are passed the current node, the InstrumentationManager, and a cursor to the current node.
// These functions are invoked on every node in the DST tree no matter what.
type StatelessTracingFunction func(n dst.Node, manager *InstrumentationManager, c *dstutil.Cursor)
