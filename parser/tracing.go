package parser

import (
	"fmt"
	"slices"

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

func (tc *tracingState) CreateTransactionIfNeeded(c *dstutil.Cursor, wrappedStatement dst.Stmt, functionName, txnVariableName string) {
	if tc.agentVariable != "" && c.Index() > 0 {
		tc.txnVariable = defaultTxnName
		start := codegen.StartTransaction(tc.agentVariable, defaultTxnName, functionName, tc.definedTxn)
		end := codegen.EndTransaction(defaultTxnName)
		codegen.WrapStatements(start, wrappedStatement, end)
		tc.definedTxn = true
		c.InsertBefore(start)
		c.InsertAfter(end)
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

type segmentOpts struct {
	async  bool
	create bool
}

// name gets the name of a segment
func (opt *segmentOpts) name(fn *dst.FuncDecl) string {
	name := fn.Name.Name
	if opt.async {
		name = fmt.Sprintf("async %s", name)
	}
	return name
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

func prependStatementToFunctionDecl(fn *dst.FuncDecl, stmt dst.Stmt) {
	if fn.Body == nil || fn.Body.List == nil {
		return
	}

	fn.Body.List = slices.Insert(fn.Body.List, 0, stmt)
}

func prependStatementToFunctionLit(fn *dst.FuncLit, stmt dst.Stmt) {
	if fn.Body == nil || fn.Body.List == nil {
		return
	}

	fn.Body.List = slices.Insert(fn.Body.List, 0, stmt)
}

// TraceFunction adds tracing to a function. This includes error capture, and passing agent metadata to relevant functions and services.
// Traces all called functions inside the current package as well.
// This function returns a FuncDecl object pointer that contains the potentially modified version of the FuncDecl object, fn, passed. If
// the bool field is true, then the function was modified, and requires a transaction most likely.
func TraceFunction(manager *InstrumentationManager, fn *dst.FuncDecl, tracing *tracingState, segment segmentOpts) (*dst.FuncDecl, bool) {
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

					// create async segment; this is a special case
					prependStatementToFunctionLit(fun, codegen.DeferSegment("async literal", txnVarName))
					c.Replace(v)
					TopLevelFunctionChanged = true
				default:
					rootPkg := manager.currentPackage
					invInfo := manager.getPackageFunctionInvocation(v.Call)
					if manager.shouldInstrumentFunction(invInfo) {
						manager.setPackage(invInfo.packageName)
						decl := manager.getDeclaration(invInfo.functionName)
						TraceFunction(manager, decl, tracing.TraceDownstreamFunction(), asyncSegment())
						manager.addTxnArgumentToFunctionDecl(decl, txnVarName)
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
				_, downstreamFunctionTraced = TraceFunction(manager, decl, tracing.TraceDownstreamFunction(), functionSegment())
				if downstreamFunctionTraced {
					manager.addTxnArgumentToFunctionDecl(decl, txnVarName)
					manager.addImport(codegen.NewRelicAgentImportPath)
				}
			}
			if manager.requiresTransactionArgument(invInfo, txnVarName) {
				tracing.CreateTransactionIfNeeded(c, v, invInfo.functionName, txnVarName)
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

	if segment.create {
		prependStatementToFunctionDecl(fn, codegen.DeferSegment(segment.name(fn), tracing.GetTransactionVariable()))
		manager.addImport(codegen.NewRelicAgentImportPath)
		TopLevelFunctionChanged = true
	}

	// update the stored declaration, marking it as traced
	decl := outputNode.(*dst.FuncDecl)
	manager.updateFunctionDeclaration(decl)
	return decl, TopLevelFunctionChanged
}
