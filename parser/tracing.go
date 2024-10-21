package parser

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/parser/tracecontext"
)

// creates a segment from the transaction variable and inserts it at the beginning of the function
func createSegment(fn *dst.FuncDecl, tracecontext tracecontext.TraceContext) {
	txn, assignment := tracecontext.Transaction()
	stmts := []dst.Stmt{codegen.DeferSegment(fn.Name.Name, txn)}

	if assignment != nil {
		stmts = append([]dst.Stmt{assignment}, stmts...)
	}
	fn.Body.List = append(stmts, fn.Body.List...)
}

// TraceFunction adds tracing to a function. This includes error capture, and passing agent metadata to relevant functions and services.
// Traces all called functions inside the current package as well.
// This function returns a FuncDecl object pointer that contains the potentially modified version of the FuncDecl object, fn, passed. If
// the bool field is true, then the function was modified, and requires a transaction most likely.
func TraceFunction(manager *InstrumentationManager, fn *dst.FuncDecl, tracecontext tracecontext.TraceContext, startSegment bool) (*dst.FuncDecl, bool) {
	TopLevelFunctionChanged := false

	if startSegment {
		createSegment(fn, tracecontext)
		manager.addImport(codegen.NewRelicAgentImportPath)
		TopLevelFunctionChanged = true
	}

	outputNode := dstutil.Apply(fn, nil, func(c *dstutil.Cursor) bool {
		n := c.Node()
		switch v := n.(type) {
		case *dst.GoStmt:
			switch fun := v.Call.Fun.(type) {
			case *dst.FuncLit:
				// get the transaction variable
				txnVarName, assign := tracecontext.Transaction()
				if assign != nil && c.Index() >= 0 {
					c.InsertBefore(assign)
				}

				// Add threaded txn to function arguments and parameters
				fun.Type.Params.List = append(fun.Type.Params.List, codegen.TxnAsParameter(txnVarName))
				v.Call.Args = append(v.Call.Args, codegen.TxnNewGoroutine(dst.NewIdent(txnVarName)))
				// add go-agent/v3/newrelic to imports
				manager.addImport(codegen.NewRelicAgentImportPath)
				// create async segment
				fun.Body.List = append([]dst.Stmt{codegen.DeferSegment("async literal", txnVarName)}, fun.Body.List...)
				// call c.Replace to replace the node with the changed code and mark that code as not needing to be traversed
				c.Replace(v)
				TopLevelFunctionChanged = true
			default:
				invInfo := manager.getInvocationInfo(v.Call)
				if invInfo != nil {
					childTraceContext := tracecontext.Pass(invInfo.decl.body, v.Call, true)
					if invInfo.doTracing() {
						rootPkg := manager.currentPackage
						manager.setPackage(invInfo.packageName)
						TraceFunction(manager, invInfo.decl.body, childTraceContext, true)
						c.Replace(v)
						TopLevelFunctionChanged = true
						manager.setPackage(rootPkg)
					}
				}
			}

		case dst.Stmt:
			downstreamFunctionTraced := false
			invInfo := manager.getInvocationInfo(v)
			if invInfo != nil {
				// always try to pass the transaction to the function
				decl := manager.getDeclaration(invInfo.functionName)
				childTraceContext := tracecontext.Pass(decl, invInfo.call, false)
				if invInfo.doTracing() {
					rootPkg := manager.currentPackage
					manager.setPackage(invInfo.packageName)
					_, ok := TraceFunction(manager, decl, childTraceContext, true)
					if ok {
						downstreamFunctionTraced = true
						manager.addImport(codegen.NewRelicAgentImportPath)
					}
					manager.setPackage(rootPkg)
				}
			}

			if !downstreamFunctionTraced {
				ok := NoticeError(manager, v, c, tracecontext)
				if ok {
					TopLevelFunctionChanged = true
				}
			}
			for _, stmtFunc := range manager.tracingFunctions.stateful {
				ok := stmtFunc(manager, v, c, tracecontext)
				if ok {
					TopLevelFunctionChanged = true
				}
			}
		}
		return true
	})

	// update the stored declaration, marking it as traced
	decl := outputNode.(*dst.FuncDecl)
	manager.updateFunctionDeclaration(decl)
	return decl, TopLevelFunctionChanged
}

// Main has its own tracing rules because we need to start our own transactions.
// There are a number of cases we do not support like async function calls
func TraceMain(manager *InstrumentationManager, fn *dst.FuncDecl, agentVariableName string) dst.Node {
	pkg := manager.getDecoratorPackage()
	txnOverwrite := false

	outputNode := dstutil.Apply(fn, nil, func(c *dstutil.Cursor) bool {
		n := c.Node()
		switch v := n.(type) {
		case *dst.GoStmt:
			comment.Info(pkg, n,
				"Go Easy Instrumentation can not trace async function calls that are not inside a function call.",
				"Please manually trace this goroutine, or move it to a function that awaits its completion.",
			)
			// do not traverse this node's children
			c.Replace(v)

		case dst.Stmt:
			invInfo := manager.getInvocationInfo(v)
			if c.Index() >= 0 && invInfo != nil {
				tracecontext, txnStmt := tracecontext.StartTransaction(pkg, defaultTxnName, invInfo.functionName, agentVariableName, txnOverwrite)
				txnOverwrite = true
				c.InsertBefore(txnStmt)
				c.InsertAfter(codegen.EndTransaction(defaultTxnName))
				childTraceContext := tracecontext.Pass(invInfo.decl.body, invInfo.call, false) // this will update the call and decl only if needed
				if invInfo.doTracing() {
					rootPkg := manager.currentPackage
					manager.setPackage(invInfo.packageName)
					_, ok := TraceFunction(manager, invInfo.decl.body, childTraceContext, false)
					if ok {
						manager.addImport(codegen.NewRelicAgentImportPath)
					}
					// restore package back to root
					manager.setPackage(rootPkg)
				}

				NoticeError(manager, v, c, tracecontext)
				for _, stmtFunc := range manager.tracingFunctions.stateful {
					stmtFunc(manager, v, c, tracecontext)
				}
			}
		}
		return true
	})

	return outputNode
}
