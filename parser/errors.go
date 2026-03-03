package parser

import (
	"fmt"
	"go/token"
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

// StatefulTracingFunction
// NoticeError will check for the presence of an error.Error variable in the body at the index in bodyIndex.
// If it finds that an error is returned, it will add a line after the assignment statement to capture an error
// with a newrelic transaction. All transactions are assumed to be named "txn"
func NoticeError(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State, functionCallWasTraced bool) bool {
	if tracing.IsMain() {
		return false
	}

	pkg := manager.GetDecoratorPackage()
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

				comment.Debug(pkg, stmt, "Capturing error return value for NoticeError")

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
			cachedExpr := manager.ErrorCache().GetExpression()
			if cachedExpr != nil && util.AssertExpressionEqual(result, cachedExpr) {
				manager.ErrorCache().Clear()
				comment.Debug(pkg, stmt, "Injecting error nil check with NoticeError before return")
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
			errExpr := manager.ErrorCache().GetExpression()
			if errExpr != nil {
				var stmtBlock dst.Stmt
				if nodeVal.Body != nil && len(nodeVal.Body.List) > 0 {
					stmtBlock = nodeVal.Body.List[0]
				}
				comment.Debug(pkg, stmt, "Injecting NoticeError into error handling block")
				nodeVal.Body.List = append([]dst.Stmt{codegen.NoticeError(errExpr, tracing.TransactionVariable(), stmtBlock)}, nodeVal.Body.List...)
				manager.ErrorCache().Clear()
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

		cachedErrExpr := manager.ErrorCache().GetExpression()
		if cachedErrExpr != nil {
			stmt := manager.ErrorCache().GetStatement()
			comment.Warn(pkg, stmt, stmt, fmt.Sprintf("Unchecked Error \"%s\", please consult New Relic documentation on error capture", util.WriteExpr(cachedErrExpr, pkg)))
			manager.ErrorCache().Clear()
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
		if !manager.ErrorCache().IsExistingError(errExpr) {
			manager.ErrorCache().Load(errExpr, errStmt)
		}
	}
	return false
}
