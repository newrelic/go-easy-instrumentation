package main

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

var RequiredStatefulTracingFunctions = []StatefulTracingFunction{ExternalHttpCall, WrapNestedHandleFunction}

// TraceFunction adds tracing to a function. This includes error capture, and passing agent metadata to relevant functions and services.
// Traces all called functions inside the current package as well.
// This function returns a FuncDecl object pointer that contains the potentially modified version of the FuncDecl object, fn, passed. If
// the bool field is true, then the function was modified, and requires a transaction most likely.
func TraceFunction(manager *InstrumentationManager, fn *dst.FuncDecl, txnVarName string) (*dst.FuncDecl, bool) {
	TopLevelFunctionChanged := false
	outputNode := dstutil.Apply(fn, nil, func(c *dstutil.Cursor) bool {
		n := c.Node()
		switch v := n.(type) {
		case *dst.GoStmt:
			switch fun := v.Call.Fun.(type) {
			case *dst.FuncLit:
				// Add threaded txn to function arguments and parameters
				fun.Type.Params.List = append(fun.Type.Params.List, txnAsParameter(txnVarName))
				v.Call.Args = append(v.Call.Args, txnNewGoroutine(txnVarName))
				// add go-agent/v3/newrelic to imports
				manager.AddImport(newrelicAgentImport)

				// create async segment
				fun.Body.List = append([]dst.Stmt{deferSegment("async literal", txnVarName)}, fun.Body.List...)
				c.Replace(v)
				TopLevelFunctionChanged = true
			default:
				rootPkg := manager.currentPackage
				invInfo := manager.GetPackageFunctionInvocation(v.Call)
				if manager.ShouldInstrumentFunction(invInfo) {
					manager.SetPackage(invInfo.packageName)
					decl := manager.GetDeclaration(invInfo.functionName)
					TraceFunction(manager, decl, txnVarName)
					manager.AddTxnArgumentToFunctionDecl(decl, txnVarName)
					manager.AddImport(newrelicAgentImport)
					decl.Body.List = append([]dst.Stmt{deferSegment(fmt.Sprintf("async %s", invInfo.functionName), txnVarName)}, decl.Body.List...)
				}
				if manager.RequiresTransactionArgument(invInfo, txnVarName) {
					invInfo.call.Args = append(invInfo.call.Args, txnNewGoroutine(txnVarName))
					c.Replace(v)
					TopLevelFunctionChanged = true
				}
				manager.SetPackage(rootPkg)

			}
		case dst.Stmt:
			downstreamFunctionTraced := false
			rootPkg := manager.currentPackage
			invInfo := manager.GetPackageFunctionInvocation(v)

			if manager.ShouldInstrumentFunction(invInfo) {
				manager.SetPackage(invInfo.packageName)
				decl := manager.GetDeclaration(invInfo.functionName)
				_, downstreamFunctionTraced = TraceFunction(manager, decl, txnVarName)
				if downstreamFunctionTraced {
					manager.AddTxnArgumentToFunctionDecl(decl, txnVarName)
					manager.AddImport(newrelicAgentImport)
					decl.Body.List = append([]dst.Stmt{deferSegment(invInfo.functionName, txnVarName)}, decl.Body.List...)
				}
			}
			if manager.RequiresTransactionArgument(invInfo, txnVarName) {
				invInfo.call.Args = append(invInfo.call.Args, dst.NewIdent(txnVarName))
				TopLevelFunctionChanged = true
			}
			manager.SetPackage(rootPkg)
			if !downstreamFunctionTraced {
				ok := NoticeError(manager, v, c, txnVarName)
				if ok {
					TopLevelFunctionChanged = true
				}
			}
			for _, stmtFunc := range RequiredStatefulTracingFunctions {
				ok := stmtFunc(manager, v, c, txnVarName)
				if ok {
					TopLevelFunctionChanged = true
				}
			}
		}
		return true
	})

	// update the stored declaration, marking it as traced
	decl := outputNode.(*dst.FuncDecl)
	manager.UpdateFunctionDeclaration(decl)
	return decl, TopLevelFunctionChanged
}
