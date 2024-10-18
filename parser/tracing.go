package parser

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
)

type tracingState struct {
	definedTxn    bool
	agentVariable string
	txnVariable   string
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

// TraceFunction adds tracing to a function. This includes error capture, and passing agent metadata to relevant functions and services.
// Traces all called functions inside the current package as well.
// This function returns a FuncDecl object pointer that contains the potentially modified version of the FuncDecl object, fn, passed. If
// the bool field is true, then the function was modified, and requires a transaction most likely.
//
// TODO: there is a ton of complexity around tracing async statements that do not have a transaction wrapping them. This is a feature gap.
func TraceFunction(manager *InstrumentationManager, fn *dst.FuncDecl, tracing *tracingState) (*dst.FuncDecl, bool) {
	TopLevelFunctionChanged := false
	outputNode := dstutil.Apply(fn, nil, func(c *dstutil.Cursor) bool {
		n := c.Node()
		switch v := n.(type) {
		case *dst.GoStmt:
			// Skip Tracing of go functions in Main. This is extremenly complicated and not implemented right now.
			// TODO: Implement this
			agentVariable := tracing.GetAgentVariable()
			if agentVariable == "" {
				txnVarName := tracing.GetTransactionVariable()
				switch fun := v.Call.Fun.(type) {
				case *dst.FuncLit:
					// Add threaded txn to function arguments and parameters
					fun.Type.Params.List = append(fun.Type.Params.List, codegen.TxnAsParameter(txnVarName))
					v.Call.Args = append(v.Call.Args, codegen.TxnNewGoroutine(txnVarName))
					// add go-agent/v3/newrelic to imports
					manager.addImport(codegen.NewRelicAgentImportPath)

					// create async segment
					fun.Body.List = append([]dst.Stmt{codegen.DeferSegment("async literal", txnVarName)}, fun.Body.List...)
					c.Replace(v)
					TopLevelFunctionChanged = true
				default:
					rootPkg := manager.currentPackage
					invInfo := manager.getPackageFunctionInvocation(v.Call)
					if manager.shouldInstrumentFunction(invInfo) {
						manager.setPackage(invInfo.packageName)
						decl := manager.getDeclaration(invInfo.functionName)
						TraceFunction(manager, decl, tracing.TraceDownstreamFunction())
						manager.addTxnArgumentToFunctionDecl(decl, txnVarName)

						manager.addImport(codegen.NewRelicAgentImportPath)
						decl.Body.List = append([]dst.Stmt{codegen.DeferSegment(fmt.Sprintf("async %s", invInfo.functionName), txnVarName)}, decl.Body.List...)
					}
					if manager.requiresTransactionArgument(invInfo, txnVarName) {
						invInfo.call.Args = append(invInfo.call.Args, codegen.TxnNewGoroutine(txnVarName))
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
