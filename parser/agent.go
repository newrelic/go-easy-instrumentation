package parser

import (
	"fmt"
	"go/token"
	"go/types"
	"slices"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

const (
	untypedNil = "untyped nil"
)

// errorReturnIndex returns the index of the error return value in the function call
// if no error is returned it will return 0, false
func errorReturnIndex(v *dst.CallExpr, pkg *decorator.Package) (int, bool) {
	if pkg == nil {
		return 0, false
	}

	ty := util.TypeOf(v, pkg)
	if ty == nil {
		return 0, false
	}

	tup, ok := ty.(*types.Tuple)
	if ok {
		for i := 0; i < tup.Len(); i++ {
			t := tup.At(i).Type()
			if util.IsError(t) {
				return i, true
			}
		}
	}

	if util.IsError(ty) {
		return 0, true
	}

	return 0, false
}

func isNewRelicMethod(call *dst.CallExpr) bool {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if ok {
		if pkg, ok := sel.X.(*dst.Ident); ok {
			return pkg.Name == "newrelic"
		}
	} else {
		if ident, ok := call.Fun.(*dst.Ident); ok {
			return ident.Path == codegen.NewRelicAgentImportPath
		}
	}
	return false
}

func findErrorVariable(stmt *dst.AssignStmt, pkg *decorator.Package) dst.Expr {
	for _, variable := range stmt.Lhs {
		t := util.TypeOf(variable, pkg)
		if t == nil {
			continue
		}

		// ignore blank identifiers
		ident, ok := variable.(*dst.Ident)
		if ok && ident.Name == "_" {
			continue
		}

		// if the variable is an error type, return it
		if util.IsError(t) {
			return variable
		}
	}
	return nil
}

// StatelessTracingFunctions
//////////////////////////////////////////////

// InstrumentMain looks for the main method of a program, and uses this as an instrumentation initialization and injection point
// TODO: Can this be refactored to be part of the Trace Function algorithm?
func InstrumentMain(manager *InstrumentationManager, c *dstutil.Cursor) {
	mainFunctionNode := c.Node()
	if decl, ok := mainFunctionNode.(*dst.FuncDecl); ok {
		// only inject go agent into the main.main function
		if decl.Name.Name == "main" {
			agentDecl := codegen.InitializeAgent(manager.appName, manager.agentVariableName)
			decl.Body.List = append(agentDecl, decl.Body.List...)
			decl.Body.List = append(decl.Body.List, codegen.ShutdownAgent(manager.agentVariableName))

			// add go-agent/v3/newrelic to imports
			manager.addImport(codegen.NewRelicAgentImportPath)
			newMain, _ := TraceFunction(manager, decl, tracestate.Main(manager.agentVariableName))

			// this will skip the tracing of this function in the outer tree walking algorithm
			c.Replace(newMain)
		}
	}
}
func findErrorVariableIf(stmt *dst.IfStmt, manager *InstrumentationManager) dst.Expr {
	if binExpr, ok := stmt.Cond.(*dst.BinaryExpr); ok {
		if exp, ok := binExpr.X.(*dst.Ident); ok {
			if exp.Obj != nil {
				if objData, ok := exp.Obj.Decl.(*dst.AssignStmt); ok {
					return findErrorVariable(objData, manager.getDecoratorPackage())
				}
			}
			return nil
		}
	}

	return nil
}

// errNilCheck tests if an if statement contains a conditional check that an error is not nil
func errNilCheck(stmt *dst.BinaryExpr, pkg *decorator.Package) bool {
	exprTypeX := util.TypeOf(stmt.X, pkg)
	if exprTypeX == nil {
		return false
	}

	exprTypeY := util.TypeOf(stmt.Y, pkg)
	if exprTypeY == nil {
		return false
	}

	// If the left side contains a nested error that checks err != nil, then return true
	nestedX, okX := stmt.X.(*dst.BinaryExpr)
	if okX && errNilCheck(nestedX, pkg) {
		return true
	}

	// If the right side contains a nested error that checks err != nil, then return true
	nestedY, okY := stmt.Y.(*dst.BinaryExpr)
	if okY && errNilCheck(nestedY, pkg) {
		return true
	}

	// base case: this is a single binary expression
	if stmt.Op != token.NEQ {
		return false
	}

	if util.IsError(exprTypeX) && exprTypeY.String() == untypedNil {
		return true
	}

	if util.IsError(exprTypeY) && exprTypeX.String() == untypedNil {
		return true
	}
	return false
}

func shouldNoticeError(stmt dst.Stmt, pkg *decorator.Package, tracing *tracestate.State) bool {
	ifStmt, ok := stmt.(*dst.IfStmt)
	if !ok {
		return false
	}
	binExpr, ok := ifStmt.Cond.(*dst.BinaryExpr)
	if ok && errNilCheck(binExpr, pkg) {
		return true
	}

	return shouldNoticeError(ifStmt.Else, pkg, tracing)
}

// StatefulTracingFunctions
//////////////////////////////////////////////

// NoticeError will check for the presence of an error.Error variable in the body at the index in bodyIndex.
// If it finds that an error is returned, it will add a line after the assignment statement to capture an error
// with a newrelic transaction. All transactions are assumed to be named "txn"
func NoticeError(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State, functionCallWasTraced bool) bool {
	if tracing.IsMain() {
		return false
	}

	pkg := manager.getDecoratorPackage()
	switch nodeVal := stmt.(type) {
	case *dst.ReturnStmt:
		if functionCallWasTraced || c.Index() < 0 {
			return false
		}
		for i, result := range nodeVal.Results {
			call, ok := result.(*dst.CallExpr)
			if ok {
				newSmts, retVals := codegen.CaptureErrorReturnCallExpression(pkg, call, tracing.TransactionVariable())
				if newSmts == nil {
					return false
				}

				// add an empty line beore the return statement for readability
				nodeVal.Decorations().Before = dst.EmptyLine

				// if this is the first element in the slice, it will be the top of the function
				if c.Index() == 0 {
					newSmts[0].Decorations().Before = dst.NewLine
				}

				for _, stmt := range newSmts {
					c.InsertBefore(stmt)
				}

				nodeVal.Results = slices.Delete(nodeVal.Results, i, i+1)
				nodeVal.Results = slices.Insert(nodeVal.Results, i, retVals...)
			}
			cachedExpr := manager.errorCache.GetExpression()
			if cachedExpr != nil && util.AssertExpressionEqual(result, cachedExpr) {
				manager.errorCache.Clear()
				capture := codegen.IfErrorNotNilNoticeError(cachedExpr, tracing.TransactionVariable())
				capture.Decs.Before = dst.EmptyLine
				c.InsertBefore(capture)
				return true
			}
		}
	case *dst.IfStmt:
		if nodeVal.Init != nil {
			NoticeError(manager, nodeVal.Init, c, tracing, functionCallWasTraced)
		}
		if shouldNoticeError(stmt, pkg, tracing) {
			errExpr := manager.errorCache.GetExpression()
			if errExpr != nil {
				var stmtBlock dst.Stmt
				if nodeVal.Body != nil && len(nodeVal.Body.List) > 0 {
					stmtBlock = nodeVal.Body.List[0]
				}
				nodeVal.Body.List = append([]dst.Stmt{codegen.NoticeError(errExpr, tracing.TransactionVariable(), stmtBlock)}, nodeVal.Body.List...)
				manager.errorCache.Clear()
				return true
			}
		}
	case *dst.AssignStmt:
		if c.Index() < 0 {
			return false
		}

		// avoid capturing errors that were already captured upstream
		if functionCallWasTraced {
			return false
		}
		// if the call was traced, ignore the assigned error because it will be captured in the upstream
		// function body
		errExpr := findErrorVariable(nodeVal, pkg)
		if errExpr == nil {
			return false
		}

		cachedErrExpr := manager.errorCache.GetExpression()
		if cachedErrExpr != nil {
			stmt := manager.errorCache.GetStatement()
			comment.Warn(pkg, stmt, stmt, fmt.Sprintf("Unchecked Error \"%s\", please consult New Relic documentation on error capture", util.WriteExpr(cachedErrExpr, pkg)))
			manager.errorCache.Clear()
		}

		// Always load the error into the cache
		var errStmt dst.Stmt
		errStmt = nodeVal

		// its possible that this error is not in a block statment
		// if thats the case, we should attempt to add our comment to something that is.
		if c.Index() < 0 {
			parent := c.Parent()
			parentStmt, ok := parent.(dst.Stmt)
			if ok {
				errStmt = parentStmt
			}
		}
		manager.errorCache.Load(errExpr, errStmt)
	}
	return false
}
