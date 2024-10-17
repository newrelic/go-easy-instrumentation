package codegen

import (
	"go/token"

	"github.com/dave/dst"
)

const DefaultContextVariableName = "newRelicContext"

// WrapContext wraps a context with a transaction and assigns it to a variable.
func WrapContext(context dst.Expr, transaction dst.Expr, newContextVariableName string) dst.Stmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{
			dst.NewIdent(newContextVariableName),
		},
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "NewContext",
					Path: NewRelicAgentImportPath,
				},
				Args: []dst.Expr{
					context,
					transaction,
				},
			},
		},
	}
}

// WrapContextExpression wraps a context with a transaction as an expression. This does not assign
// the result to a variable.
// This is for use in a function call argument.
func WrapContextExpression(context dst.Expr, transaction dst.Expr) dst.Expr {
	return &dst.CallExpr{
		Fun: &dst.Ident{
			Name: "NewContext",
			Path: NewRelicAgentImportPath,
		},
		Args: []dst.Expr{
			context,
			transaction,
		},
	}
}

func IsWrapContextExpression(expr dst.Expr) bool {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}
	if call.Fun == nil {
		return false
	}
	ident, ok := call.Fun.(*dst.Ident)
	if !ok {
		return false
	}
	return ident.Name == "NewContext" && ident.Path == NewRelicAgentImportPath
}

// TxnFromContext creates an assignment statement that extracts a transaction from a context.
func TxnFromContext(txnVariable string, contextObject dst.Expr) *dst.AssignStmt {
	return &dst.AssignStmt{
		Decs: dst.AssignStmtDecorations{
			NodeDecs: dst.NodeDecs{
				After: dst.EmptyLine,
			},
		},
		Lhs: []dst.Expr{
			&dst.Ident{
				Name: txnVariable,
			},
		},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "FromContext",
					Path: NewRelicAgentImportPath,
				},
				Args: []dst.Expr{
					dst.Clone(contextObject).(dst.Expr),
				},
			},
		},
	}
}

// TxnFromContextExpression creates an expression that extracts a transaction from a context.
// This is for use in a function call argument.
func TxnFromContextExpression(contextObject dst.Expr) dst.Expr {
	return &dst.CallExpr{
		Fun: &dst.Ident{
			Name: "FromContext",
			Path: NewRelicAgentImportPath,
		},
		Args: []dst.Expr{
			dst.Clone(contextObject).(dst.Expr),
		},
	}
}

func IsTxnFromContextExpression(expr dst.Expr) bool {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}
	if call.Fun == nil {
		return false
	}
	ident, ok := call.Fun.(*dst.Ident)
	if !ok {
		return false
	}
	return ident.Name == "FromContext" && ident.Path == NewRelicAgentImportPath
}
