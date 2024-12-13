/*
Parser is a static analysis library that can detect and inject instrumentation code into a Go application.

It does this in the following steps.

1. Generate an abstract syntax tree from the application using DST. A unique tree will be generated for every package in the parsed application. Trees
get stored in a cache that seperates data based on the pacakage it belongs to, and can be looked up by the path of that package, which is a unique identifier.

2. Walk the syntax tree for each package in a given application. While we do that, we build a new data structure that contains data mapped by package. This data structure is looking for a few key pieces of information, but primarily, it is looking for user defined function declarations. These declarations are objects in the tree, and we can uniquely identify them by package name, and function name. Additional key information is discovered with `FactDiscoveryFunctions` and cached in an object called the `FactStore`, which can be used for recognizing key information that is not available in the scope of a single package or function call.

3. Once we have gathered all our facts and impelentation data, we have all the information we need to instrument an application. The tool will walk through the entire syntax tree for each package again, making this the second full walk of the tree(s). This time, it will look for sections of code where middleware can be injected, tracing has already been started by middleware, or tracing could potentially be started using `StatelessTracingFunctions`. Then it will apply tracing to that section of code, as well as all reachable code that is called from the current scope, using `StatefulTracingFunctions`. Once this has completed, a modified tree with complete instrumentation written into the code has been built.

4. Restore the modified tree back to code. That code is compared to the original application, and a GIT compatible diff is generated in memory. This diff file is written to a file in the local operating system where the user can review it and decide how to proceed.
*/
package parser

import (
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/parser/facts"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

// StatefulTracingFunction defines a function that requires knowledge of the state of New Relic tracing
// in the current scope of the application in order to apply its changes. That state is stored in the
// `tracestate.State` object. `StatefulTracingFunctions` are executed against every line of code in
// the body of a function being traced, as well as every line of code in functions that are declard
// in this application and called by the function being traced.
//
// The `stmt` is the line of code that is currently being analyzed in the body of a function being traced.
// This should always be the same node as the current node in the `dstutil.Cursor`. The cursor is provided
// to allow for easy modifications to the AST tree. The `tracing` manages the current state of New
// Relic tracing in the application, and has a number of methods that can be used to easily access or apply
// tracing to the current code.
//
// If the `stmt` was modified, it should return true, otherwise false.
type StatefulTracingFunction func(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool

// StatelessTracingFunction are a powerful tool for identifying and modifying specific sections of code.
// These functions operate independently, without needing information about the current scope of the code
// they analyze, the Go agent application, Go agent transactions, or any prior modifications to the code.
// They are particularly effective in detecting code segments suitable for middleware injection or initiating
// tracing when middleware is already present.
//
// These functions are ideal for scenarios where a consistent operation can be applied to a specific code
// pattern. Stateless Tracing Functions are loaded into the manager during initialization and are executed
// during the second traversal of the abstract syntax tree (AST).
//
// These functions are passed the current node, and a cursor to the current node.
// These functions are invoked on every node in the DST tree.
type StatelessTracingFunction func(manager *InstrumentationManager, c *dstutil.Cursor)

// FactDiscoveryFunction identify a "Fact" about a code pattern, which can be referenced later to identify
// patterns that are essential for instrumentation. This function is executed on all nodes in the syntax tree
// of every function declared in an application. Facts are deterministic labels assigned to specific patterns.
// When a FactDiscoveryFunction identifies a fact in a node of the abstract syntax tree (AST), it should
// return a fact `Entry“ for the manager to cache for future use and a boolean indicating if the fact was found.
//
// These functions are best used when a piece of information must be known about the application
// in order for some tracing functions to work, and we can not determine that information from the
// scope those functions have access to.
type FactDiscoveryFunction func(pkg *decorator.Package, node dst.Node) (facts.Entry, bool)
