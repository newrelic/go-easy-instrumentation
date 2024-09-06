package parser

import (
	"go/ast"
	"go/types"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/parser/codegen"
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

			newMain, _ := TraceFunction(manager, decl, TraceMain(manager.agentVariableName, defaultTxnName))

			// this will skip the tracing of this function in the outer tree walking algorithm
			c.Replace(newMain)
		}
	}
}

// StatefulTracingFunctions
//////////////////////////////////////////////

// NoticeError will check for the presence of an error.Error variable in the body at the index in bodyIndex.
// If it finds that an error is returned, it will add a line after the assignment statement to capture an error
// with a newrelic transaction. All transactions are assumed to be named "txn"
func NoticeError(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracingState) bool {
	switch nodeVal := stmt.(type) {
	case *dst.AssignStmt:
		errExpr := findErrorVariable(nodeVal, manager.getDecoratorPackage())
		if errExpr != nil && c.Index() >= 0 {
			c.InsertAfter(codegen.NoticeError(errExpr, tracing.txnVariable, nodeVal.Decorations()))
			return true
		}
	}
	return false
}
