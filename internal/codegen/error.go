package codegen

import (
	"fmt"
	"go/token"
	"go/types"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/common"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
)

// IfErrorNotNilNoticeError creates an if statement that checks if the errorVariable is not nil, and calls notice error if its not nil
//
// Example:
//
//	if err != nil {
//		txn.NoticeError(err)
//	}
func IfErrorNotNilNoticeError(errorVariable, transactionVariable dst.Expr) *dst.IfStmt {
	return &dst.IfStmt{
		Cond: &dst.BinaryExpr{
			X:  dst.Clone(errorVariable).(dst.Expr),
			Op: token.NEQ,
			Y:  dst.NewIdent("nil"),
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				NoticeError(dst.Clone(errorVariable).(dst.Expr), transactionVariable, nil),
			},
		},
	}
}

// NoticeError Generates a statement that calls txn.NoticeError(err)
func NoticeError(errExpr, transactionVariable dst.Expr, stmtBlock dst.Stmt) *dst.ExprStmt {
	var decs dst.ExprStmtDecorations
	// copy all decs below the current statement into this statement
	if stmtBlock != nil {
		decs.Before = stmtBlock.Decorations().Before
		decs.Start = stmtBlock.Decorations().Start
		stmtBlock.Decorations().Before = dst.None
		stmtBlock.Decorations().Start.Clear()
	}

	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X: transactionVariable,
				Sel: &dst.Ident{
					Name: "NoticeError",
				},
			},
			Args: []dst.Expr{dst.Clone(errExpr).(dst.Expr)},
		},
		Decs: decs,
	}
}

// CaptureErrorReturnCallExpression checks if the return values of a function call is an error, and generates code to assign the return values to variables,
// check if the error is not nil, and call txn.NoticeError(err) if the error is not nil.
// It returns the statements that need to be added to the tree, and the expressions that are assigned to the return values of the function call.
// The list of expressions can be used to replace the expression in the return statement.
func CaptureErrorReturnCallExpression(pkg *decorator.Package, call *dst.CallExpr, transactionVariable dst.Expr) ([]dst.Stmt, []dst.Expr) {
	t := util.TypeOf(call, pkg)
	if t == nil {
		return nil, nil
	}

	// numReturnVariables is the number of return variables in the function call
	numReturnVariables := 1
	// errorIndex is the index of the error return value in the function call
	errorIndex := -1

	returnTypes := []string{}
	// find the index of the error return value
	tuple, ok := t.(*types.Tuple)
	if ok {
		numReturnVariables = tuple.Len()

		for i := 0; i < tuple.Len(); i++ {
			tupleValue := tuple.At(i).Type()
			if tupleValue == nil {
				continue
			}

			returnTypes = append(returnTypes, tupleValue.String())
			if util.IsError(tupleValue) {
				errorIndex = i
			}
		}
	} else {
		returnTypes = append(returnTypes, t.String())
		if util.IsError(t) {
			errorIndex = 0
		}
	}

	if errorIndex == -1 {
		return nil, nil
	}

	typesHeader := fmt.Sprintf("// generated by %s; ", common.ApplicationName)
	variableAssignments := make([]dst.Expr, numReturnVariables)
	assignmentReturns := make([]dst.Expr, numReturnVariables) // this is a duplicate of variableAssignments so it can be used again in the tree
	for indx := range variableAssignments {
		variableAssignments[indx] = dst.NewIdent(fmt.Sprintf("returnValue%d", indx))
		assignmentReturns[indx] = dst.NewIdent(fmt.Sprintf("returnValue%d", indx))
		typesHeader += fmt.Sprintf("returnValue%d:%s", indx, returnTypes[indx])
		if indx != numReturnVariables-1 {
			typesHeader += ", "
		}
	}

	assignStmt := &dst.AssignStmt{
		Lhs: variableAssignments,
		Tok: token.DEFINE,
		Rhs: []dst.Expr{dst.Clone(call).(*dst.CallExpr)},
		Decs: dst.AssignStmtDecorations{
			NodeDecs: dst.NodeDecs{
				Before: dst.EmptyLine,
				Start:  dst.Decorations{typesHeader},
			},
		},
	}
	errCapture := IfErrorNotNilNoticeError(variableAssignments[errorIndex], transactionVariable)
	retStmts := []dst.Stmt{assignStmt, errCapture}

	return retStmts, assignmentReturns
}
