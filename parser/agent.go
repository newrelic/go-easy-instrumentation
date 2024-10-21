package parser

import (
	"go/ast"
	"go/types"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
)

const (
	UntypedNil = "untyped nil"
)

func isNamedError(n *types.Named) bool {
	if n == nil {
		return false
	}

	o := n.Obj()
	return o != nil && o.Pkg() == nil && o.Name() == "error"
}

// errorReturnIndex returns the index of the error return value in the function call
// if no error is returned it will return 0, false
func errorReturnIndex(v *dst.CallExpr, pkg *decorator.Package) (int, bool) {
	if pkg == nil {
		return 0, false
	}

	astCall, ok := pkg.Decorator.Ast.Nodes[v]
	if ok {
		ty := pkg.TypesInfo.TypeOf(astCall.(*ast.CallExpr))
		switch n := ty.(type) {
		case *types.Named:
			if isNamedError(n) {
				return 0, true
			}
		case *types.Tuple:
			for i := 0; i < n.Len(); i++ {
				t := n.At(i).Type()
				switch e := t.(type) {
				case *types.Named:
					if isNamedError(e) {
						return i, true
					}
				}
			}
		}
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
	if len(stmt.Rhs) == 1 {
		if call, ok := stmt.Rhs[0].(*dst.CallExpr); ok {
			if !isNewRelicMethod(call) {
				errIndex, ok := errorReturnIndex(call, pkg)
				if ok {
					expr := stmt.Lhs[errIndex]
					ident, ok := expr.(*dst.Ident)
					if ok {
						// ignored errors are ignored by instrumentation as well
						if ident.Name == "_" {
							return nil
						}
					}
					return dst.Clone(expr).(dst.Expr)
				}
			}
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

			newMain, _ := TraceFunction(manager, decl, TraceMain(manager.agentVariableName, defaultTxnName), noSegment())

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

func errNilCheck(stmt *dst.BinaryExpr, pkg *decorator.Package) bool {

	exprTypeX := util.TypeOf(stmt.X, pkg)
	exprTypeY := util.TypeOf(stmt.Y, pkg)
	// Case - err != nil && condition
	// If there is an extra condition, the types of X and Y will be booleans
	nestedX, okX := stmt.X.(*dst.BinaryExpr)
	nestedY, okY := stmt.Y.(*dst.BinaryExpr)

	if okX && okY {
		return errNilCheck(nestedX, pkg) || errNilCheck(nestedY, pkg)
	}

	if okX {
		return errNilCheck(nestedX, pkg)
	}

	if okY {
		return errNilCheck(nestedY, pkg)
	}
	if exprTypeX != nil && exprTypeX.String() == "error" {
		if exprTypeY != nil && exprTypeY.String() == UntypedNil {
			return true
		}
	}
	if exprTypeY != nil && exprTypeY.String() == "error" {
		if exprTypeX != nil && exprTypeX.String() == UntypedNil {
			return true
		}
	}
	return false
}

func shouldNoticeError(stmt dst.Stmt, pkg *decorator.Package, tracing *tracingState) bool {
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
func NoticeError(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracingState) bool {
	switch nodeVal := stmt.(type) {
	case *dst.IfStmt:
		if shouldNoticeError(stmt, manager.getDecoratorPackage(), tracing) {
			errExpr := manager.errorCache.GetExpression()
			if errExpr != nil {
				var stmtBlock dst.Stmt
				if nodeVal.Body != nil && len(nodeVal.Body.List) > 0 {
					stmtBlock = nodeVal.Body.List[0]
				}
				nodeVal.Body.List = append([]dst.Stmt{codegen.NoticeError(errExpr, tracing.txnVariable, stmtBlock)}, nodeVal.Body.List...)
				manager.errorCache.Clear()
				return true
			}
		}
	case *dst.AssignStmt:
		errExpr := findErrorVariable(nodeVal, manager.getDecoratorPackage())
		if errExpr != nil {
			if manager.errorCache.GetExpression() != nil {
				stmt := manager.errorCache.GetStatement()
				comment.Warn(manager.getDecoratorPackage(), stmt, "Unchecked Error, please consult New Relic documentation on error capture")

				manager.errorCache.Clear()
				return true
			} else {
				manager.errorCache.Load(errExpr, nodeVal)
			}
		}
	}
	return false
}
