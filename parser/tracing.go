package parser

import (
	"fmt"
	"reflect"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/internal/common"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

// TraceFunction adds tracing to a function. This includes error capture, and passing agent metadata to relevant functions and services.
// Traces all called functions inside the current package as well.
// This function returns a FuncDecl object pointer that contains the potentially modified version of the FuncDecl object, fn, passed. If
// the bool field is true, then the function was modified, and requires a transaction most likely.
//
// This function can accept a FuncDecl or FuncLit object for the node only.
func TraceFunction(manager *InstrumentationManager, node dst.Node, tracing *tracestate.State) (dst.Node, bool) {
	TopLevelFunctionChanged := false

	nodeType := reflect.TypeOf(node)
	if nodeType != reflect.TypeOf(&dst.FuncDecl{}) && nodeType != reflect.TypeOf(&dst.FuncLit{}) {
		panic(fmt.Sprintf("TraceFunction only accepts *dst.FuncDecl or *dst.FuncLit, got %s", nodeType))
	}

	// add needed tracing object to function declaration parameters
	// this must be the first thing done, since it double checks the type assignment for tracing and will change it
	tracingImport, ok := tracing.AddParameterToDeclaration(manager.getDecoratorPackage(), node)
	if ok {
		manager.addImport(tracingImport)
		TopLevelFunctionChanged = true
	}

	// create segment if needed
	segmentImport, ok := tracing.CreateSegment(node)
	if ok {
		manager.addImport(segmentImport)
		TopLevelFunctionChanged = true
	}

	outputNode := dstutil.Apply(node, func(c *dstutil.Cursor) bool {
		n := c.Node()
		switch v := n.(type) {
		case *dst.BlockStmt, *dst.ForStmt:
			return true
		case *dst.GoStmt:
			if tracing.IsMain() {
				comment.Info(manager.getDecoratorPackage(), v, v, fmt.Sprintf("%s doesn't support tracing goroutines in a main method; please instrument manually.", common.ApplicationName), "https://docs.newrelic.com/docs/apm/agents/go-agent/instrumentation/instrument-go-transactions/#goroutines")
				return false
			}
			switch fun := v.Call.Fun.(type) {
			case *dst.FuncLit:
				pkg := manager.getDecoratorPackage()
				// Add threaded txn to function arguments and parameters
				childState, tracingImport := tracing.AddToCall(pkg, v.Call, true)
				manager.addImport(tracingImport)

				// trace the function literal body
				newFuncLit, _ := TraceFunction(manager, fun, childState)

				v.Call.Fun = newFuncLit.(*dst.FuncLit)
				c.Replace(v)

				TopLevelFunctionChanged = true

				// Return false so that the func literal body is not traced again.
				// This prevents any of the children of this node from being traversed,
				// and we already traced this func lit by calling TraceFunction on it recursively.
				return false

			default:
				rootPkg := manager.currentPackage
				invInfo := manager.findInvocationInfo(v.Call, tracing)
				if invInfo != nil {
					childState, tracingImport := tracing.AddToCall(manager.getDecoratorPackage(), v.Call, true)
					manager.addImport(tracingImport)
					c.Replace(v)
					TopLevelFunctionChanged = true

					if manager.shouldInstrumentFunction(invInfo) {
						manager.setPackage(invInfo.packageName)
						TraceFunction(manager, invInfo.decl, childState)
						manager.setPackage(rootPkg)
					}
				}
			}

		case dst.Stmt:
			downstreamFunctionTraced := false
			assign, ok := v.(*dst.AssignStmt)
			if ok && len(assign.Rhs) == 1 && len(assign.Lhs) == 1 {
				lit, ok := assign.Rhs[0].(*dst.FuncLit)
				if ok {
					pkg := manager.getDecoratorPackage()

					// radical dude :D
					litState := tracing.FuncLiteralDeclaration(pkg, lit)
					tracing.NoticeFuncLiteralAssignment(pkg, assign.Lhs[0], lit)
					TraceFunction(manager, lit, litState)
					c.Replace(v)

					// Do not do any further tracing on this node or its children
					return false
				}
			}

			rootPkg := manager.currentPackage
			invInfo := manager.findInvocationInfo(v, tracing)

			// inv info will be nil if the function is not declared in this application
			if invInfo != nil {
				tracing.WrapWithTransaction(c, invInfo.functionName, codegen.DefaultTransactionVariable) // if a trasaction needs to be created, it will be created here
				childState, tracingImport := tracing.AddToCall(manager.getDecoratorPackage(), invInfo.call, false)
				manager.addImport(tracingImport)
				TopLevelFunctionChanged = true

				if manager.shouldInstrumentFunction(invInfo) {
					manager.setPackage(invInfo.packageName)
					TraceFunction(manager, invInfo.decl, childState)
					downstreamFunctionTraced = true
					manager.setPackage(rootPkg)
				}
			}

			ok = NoticeError(manager, v, c, tracing, downstreamFunctionTraced)
			if ok {
				TopLevelFunctionChanged = true
			}

			for _, stmtFunc := range manager.tracingFunctions.stateful {
				ok := stmtFunc(manager, v, c, tracing)
				if ok {
					TopLevelFunctionChanged = true
				}
			}
		}
		return true
	}, nil)

	// Add an assignment for txn Variable if needed
	assignmentImport := tracing.AssignTransactionVariable(node)
	manager.addImport(assignmentImport)

	// Check if error cache is still full, if so add unchecked error warning
	if manager.errorCache.GetExpression() != nil {
		stmt := manager.errorCache.GetStatement()
		comment.Warn(manager.getDecoratorPackage(), stmt, stmt, "Unchecked Error, please consult New Relic documentation on error capture", "https://docs.newrelic.com/docs/apm/agents/go-agent/api-guides/guide-using-go-agent-api/#errors")
		manager.errorCache.Clear()
	}

	// update the stored declaration, marking it as traced
	if nodeType == reflect.TypeOf(&dst.FuncDecl{}) {
		manager.updateFunctionDeclaration(outputNode.(*dst.FuncDecl))
	}

	return outputNode, TopLevelFunctionChanged
}
