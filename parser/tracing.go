package parser

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/internal/common"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

type segmentOpts struct {
	async  bool
	create bool
}

// noSegment indicates that a segment should not be created
func noSegment() segmentOpts {
	return segmentOpts{}
}

// async segment creates a segment for an async function that begins and ends in the body of a function
func asyncSegment() segmentOpts {
	return segmentOpts{async: true, create: true}
}

// functionSegment creates a segment that begins and ends in the body of a function
func functionSegment() segmentOpts {
	return segmentOpts{create: true}
}

// TraceFunction adds tracing to a function. This includes error capture, and passing agent metadata to relevant functions and services.
// Traces all called functions inside the current package as well.
// This function returns a FuncDecl object pointer that contains the potentially modified version of the FuncDecl object, fn, passed. If
// the bool field is true, then the function was modified, and requires a transaction most likely.
func TraceFunction(manager *InstrumentationManager, fn *dst.FuncDecl, tracing *tracestate.State) (*dst.FuncDecl, bool) {
	TopLevelFunctionChanged := false

	// create segment if needed
	segmentImport, ok := tracing.CreateSegment(fn)
	if ok {
		manager.addImport(segmentImport)
		TopLevelFunctionChanged = true
	}

	outputNode := dstutil.Apply(fn, nil, func(c *dstutil.Cursor) bool {
		n := c.Node()
		switch v := n.(type) {
		case *dst.GoStmt:
			if tracing.IsMain() {
				comment.Info(manager.getDecoratorPackage(), v, fmt.Sprintf("%s doesn't support tracing goroutines in a main method; please instrument manually.", common.ApplicationName), "https://docs.newrelic.com/docs/apm/agents/go-agent/instrumentation/instrument-go-transactions/#goroutines")
			} else {
				switch fun := v.Call.Fun.(type) {
				case *dst.FuncLit:
					pkg := manager.getDecoratorPackage()
					// Add threaded txn to function arguments and parameters
					tracing.AddToFunctionLiteral(pkg, fun)
					tracingImport := tracing.AddToCall(pkg, v.Call, true)
					// we can use just one import since they will be the same, and be in the same scope
					manager.addImport(tracingImport)

					// create async segment; this is a special case
					codegen.PrependStatementToFunctionLit(fun, codegen.DeferSegment("async literal", tracing.TransactionVariable()))
					c.Replace(v)
					TopLevelFunctionChanged = true
				default:
					rootPkg := manager.currentPackage
					invInfo := manager.getPackageFunctionInvocation(v.Call)
					if manager.shouldInstrumentFunction(invInfo) {
						manager.setPackage(invInfo.packageName)
						decl := manager.getDeclaration(invInfo.functionName)
						TraceFunction(manager, decl, tracing.Goroutine())
						tracingImport := tracing.AddToFunctionDecl(manager.getDecoratorPackage(), decl)
						manager.addImport(tracingImport)
						manager.setPackage(rootPkg)
					}

					// inv info will be nil if the function is not declared in this application
					if invInfo != nil {
						tracingImport := tracing.AddToCall(manager.getDecoratorPackage(), v.Call, true)
						manager.addImport(tracingImport)
						c.Replace(v)
						TopLevelFunctionChanged = true
					}
				}
			}
		case dst.Stmt:
			downstreamFunctionTraced := false
			rootPkg := manager.currentPackage
			invInfo := manager.getPackageFunctionInvocation(v)
			if manager.shouldInstrumentFunction(invInfo) {
				manager.setPackage(invInfo.packageName)
				decl := manager.getDeclaration(invInfo.functionName)
				TraceFunction(manager, decl, tracing.FunctionCall())
				tracingImport := tracing.AddToFunctionDecl(manager.getDecoratorPackage(), decl)
				manager.addImport(tracingImport)
				downstreamFunctionTraced = true
				manager.setPackage(rootPkg)
			}

			// inv info will be nil if the function is not declared in this application
			if invInfo != nil {
				tracing.WrapWithTransaction(c, invInfo.functionName, codegen.DefaultTransactionVariable) // if a trasaction needs to be created, it will be created here
				tracingImport := tracing.AddToCall(manager.getDecoratorPackage(), invInfo.call, false)
				manager.addImport(tracingImport)
				TopLevelFunctionChanged = true
			}

			// We know that if the function is traced, the error will be captured in that function.
			// In this case, we skip capturing the returned error to avoid a duplicate.
			if !downstreamFunctionTraced {
				ok := NoticeError(manager, v, c, tracing)
				if ok {
					TopLevelFunctionChanged = true
				}
			}
			for _, stmtFunc := range manager.tracingFunctions.stateful {
				ok := stmtFunc(manager, v, c, tracing)
				if ok {
					TopLevelFunctionChanged = true
				}
			}
		}
		return true
	})

	assignmentImport := tracing.AssignTransactionVariable(fn)
	manager.addImport(assignmentImport)

	// Check if error cache is still full, if so add unchecked error warning
	if manager.errorCache.GetExpression() != nil {
		comment.Warn(manager.getDecoratorPackage(), manager.errorCache.GetStatement(), "Unchecked Error, please consult New Relic documentation on error capture", "https://docs.newrelic.com/docs/apm/agents/go-agent/api-guides/guide-using-go-agent-api/#errors")
		manager.errorCache.Clear()
	}

	// update the stored declaration, marking it as traced
	decl := outputNode.(*dst.FuncDecl)
	manager.updateFunctionDeclaration(decl)
	return decl, TopLevelFunctionChanged
}
