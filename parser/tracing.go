package parser

import (
	"log"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/parser/tracecontext"
)

type tracingState struct {
	definedTxn    bool
	agentVariable string
	txnVariable   string
	txnCarrier    tracecontext.TraceContext
}

func TraceMain(agentVariable, txnVariableName string) *tracingState {
	return &tracingState{
		definedTxn:    false,
		agentVariable: agentVariable,
		txnVariable:   txnVariableName,
	}
}

func TraceDownstreamFunction(txnVariableName string) *tracingState {
	return &tracingState{
		txnVariable: txnVariableName,
	}
}

func (tc *tracingState) CreateTransactionIfNeeded(c *dstutil.Cursor, functionName, txnVariableName string, endImmediately bool) {
	if tc.agentVariable != "" && c.Index() > 0 {
		tc.txnVariable = defaultTxnName
		c.InsertBefore(codegen.StartTransaction(tc.agentVariable, defaultTxnName, functionName, tc.definedTxn))
		tc.definedTxn = true
		if endImmediately {
			c.InsertAfter(codegen.EndTransaction(defaultTxnName))
		}
	}
}

func (tc *tracingState) GetTransactionVariable() string {
	return tc.txnVariable
}

func (tc *tracingState) GetAgentVariable() string {
	return tc.agentVariable
}

func (tc *tracingState) TraceDownstreamFunction() *tracingState {
	return &tracingState{
		txnVariable: tc.txnVariable,
	}
}

func createSegment(fn *dst.FuncDecl, tracecontext tracecontext.TraceContext) {
	txnAssignment := tracecontext.AssignTransactionVariable(defaultTxnName)
	txnVariable := tracecontext.TransactionVariableName()
	stmts := []dst.Stmt{codegen.DeferSegment(fn.Name.Name, txnVariable)}

	if txnAssignment != nil {
		stmts = append([]dst.Stmt{txnAssignment}, stmts...)
	}
	fn.Body.List = append(stmts, fn.Body.List...)
}

// TraceFunction adds tracing to a function. This includes error capture, and passing agent metadata to relevant functions and services.
// Traces all called functions inside the current package as well.
// This function returns a FuncDecl object pointer that contains the potentially modified version of the FuncDecl object, fn, passed. If
// the bool field is true, then the function was modified, and requires a transaction most likely.
//
// TODO: there is a ton of complexity around tracing async statements that do not have a transaction wrapping them. This is a feature gap.
func TraceFunction(manager *InstrumentationManager, fn *dst.FuncDecl, tracecontext tracecontext.TraceContext, isMain bool, startSegment bool) (*dst.FuncDecl, bool) {
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
			// Skip Tracing of go functions in Main. This is extremenly complicated and not implemented right now.
			// TODO: Implement this
			if !isMain {
				txnVarName := tracecontext.TransactionVariableName()
				switch fun := v.Call.Fun.(type) {
				case *dst.FuncLit:
					// Add threaded txn to function arguments and parameters
					fun.Type.Params.List = append(fun.Type.Params.List, codegen.TxnAsParameter(txnVarName))
					v.Call.Args = append(v.Call.Args, codegen.TxnNewGoroutine(txnVarName))
					// add go-agent/v3/newrelic to imports
					manager.addImport(codegen.NewRelicAgentImportPath)

					// create async segment
					fun.Body.List = append([]dst.Stmt{codegen.DeferSegment("async literal", txnVarName)}, fun.Body.List...)
					// call c.Replace to replace the node with the changed code and mark that code as not needing to be traversed
					c.Replace(v)
					TopLevelFunctionChanged = true
				default:
					rootPkg := manager.currentPackage
					invInfo := manager.getPackageFunctionInvocation(v.Call)
					if manager.shouldInstrumentFunction(invInfo) {
						manager.setPackage(invInfo.packageName)
						decl := manager.getDeclaration(invInfo.functionName)
						tracecontext.AddParam(decl)
						passedContext, err := tracecontext.Pass(decl, v.Call, c, true)
						if err != nil {
							log.Printf("Failed to pass New Relic Transaction to function %s: %v", invInfo.functionName, err)
							break // skip instrumentation of this function
						}
						TraceFunction(manager, decl, passedContext, false, true)
						c.Replace(v)
						TopLevelFunctionChanged = true
					}
					manager.setPackage(rootPkg)
				}
			}
		case dst.Stmt:
			downstreamFunctionTraced := false
			rootPkg := manager.currentPackage
			invInfo := manager.getPackageFunctionInvocation(v)
			txnVarName := tracing.GetTransactionVariable()
			if manager.shouldInstrumentFunction(invInfo) {
				manager.setPackage(invInfo.packageName)
				decl := manager.getDeclaration(invInfo.functionName)
				_, downstreamFunctionTraced = TraceFunction(manager, decl, tracing.TraceDownstreamFunction())
				if downstreamFunctionTraced {
					manager.addTxnArgumentToFunctionDecl(decl, txnVarName)
					manager.addImport(codegen.NewRelicAgentImportPath)
					if tracing.agentVariable == "" {
						decl.Body.List = append([]dst.Stmt{codegen.DeferSegment(invInfo.functionName, txnVarName)}, decl.Body.List...)
					}
				}
			}
			if manager.requiresTransactionArgument(invInfo, txnVarName) {
				tracing.CreateTransactionIfNeeded(c, invInfo.functionName, txnVarName, true)
				invInfo.call.Args = append(invInfo.call.Args, dst.NewIdent(txnVarName))
				TopLevelFunctionChanged = true
			}
			manager.setPackage(rootPkg)
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

	// update the stored declaration, marking it as traced
	decl := outputNode.(*dst.FuncDecl)
	manager.updateFunctionDeclaration(decl)
	return decl, TopLevelFunctionChanged
}
