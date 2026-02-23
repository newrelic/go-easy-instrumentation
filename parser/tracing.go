package parser

import (
	"fmt"
	"reflect"
	"slices"

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

	var funcType *dst.FuncType
	var funcBody *dst.BlockStmt

	// Determine the function type and body based on node type
	decl, isFuncDecl := node.(*dst.FuncDecl)
	lit, isFuncLit := node.(*dst.FuncLit)

	if isFuncDecl {
		comment.Debug(manager.getDecoratorPackage(), node, fmt.Sprintf("TraceFunction called for function decl: %s", decl.Name.Name))
		funcType = decl.Type
		funcBody = decl.Body
	} else if isFuncLit {
		funcType = lit.Type
		funcBody = lit.Body
	} else {
		// This should not happen due to the initial type check, but it's a safeguard
		return node, false
	}

	// Check if the function already has a transaction parameter
	if !manager.hasTransactionParameter(funcType) {
		tracingImport, ok := tracing.AddParameterToDeclaration(manager.getDecoratorPackage(), node)
		if ok {
			manager.addImport(tracingImport)
			TopLevelFunctionChanged = true
		}
	}

	// Check if a segment already exists within the transaction's lifespan
	hasSegment := false
	dstutil.Apply(funcBody, func(c *dstutil.Cursor) bool {
		callExpr, ok := c.Node().(*dst.CallExpr)
		if !ok {
			return true
		}

		selExpr, ok := callExpr.Fun.(*dst.SelectorExpr)
		if !ok || selExpr.Sel.Name != "StartSegment" {
			return true
		}

		ident, ok := selExpr.X.(*dst.Ident)
		if !ok {
			return true
		}

		if manager.transactionCache.CheckTransactionExists(ident) {
			comment.Debug(manager.getDecoratorPackage(), node, fmt.Sprintf("Found existing instrumentation for function: %s", ident.Name))
			hasSegment = true
			return false // Stop further traversal
		}

		return true
	}, nil)

	// Create segment if needed
	if !hasSegment {
		segmentImport, ok := tracing.CreateSegment(node)
		if ok {
			manager.addImport(segmentImport)
			TopLevelFunctionChanged = true
		}
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
				tracableInvocations := manager.findInvocationInfo(v.Call, tracing)
				for _, invInfo := range tracableInvocations {
					childState, tracingImport := tracing.AddToCall(manager.getDecoratorPackage(), v.Call, true)
					manager.addImport(tracingImport)
					c.Replace(v)
					TopLevelFunctionChanged = true

					if manager.shouldInstrumentFunction(invInfo) {
						manager.setPackage(invInfo.packageName)
						comment.Debug(manager.getDecoratorPackage(), node, fmt.Sprintf("Tracing function: %s", invInfo.decl.Name.Name))
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
			tracableInvocations := manager.findInvocationInfo(v, tracing)
			transactionCreatedForStatement := false // prevent multiple transactions from being created for the same statement

			// inv info will be nil if the function is not declared in this application
			for _, invInfo := range tracableInvocations {
				// If the current function is the function that declares the NR App, we do not want to propagate tracing to it
				// Additionally, if the function is already being traced by an existing transaction we can skip it
				if manager.setupFunc == invInfo.decl || manager.transactionCache.IsFunctionInTransactionScope(invInfo.functionName) {
					continue
				}

				if !transactionCreatedForStatement {
					// Check if the functionName is already present within transactions
					tracing.WrapWithTransaction(c, invInfo.functionName, codegen.DefaultTransactionVariable)
					transactionCreatedForStatement = true
				}
				childState, tracingImport := tracing.AddToCall(manager.getDecoratorPackage(), invInfo.call, false)
				manager.addImport(tracingImport)
				TopLevelFunctionChanged = true
				// If not present, wrap the function with a transaction
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

// hasTransactionParameter checks if a function has a transaction parameter
// by examining the function's parameter list for any parameter names that exist
// in the transaction cache.
func (m *InstrumentationManager) hasTransactionParameter(funcType *dst.FuncType) bool {
	if funcType == nil || funcType.Params == nil {
		return false
	}

	for _, param := range funcType.Params.List {
		if slices.ContainsFunc(param.Names, m.transactionCache.CheckTransactionExists) {
			return true
		}
	}
	return false
}
