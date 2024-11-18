package codegen

import "github.com/dave/dst"

const DefaultContextParameter = "ctx"

// NewContextExpression creates an expression that creates a new context
// this is protected from using the same object, and will always clone inputs
func NewContextExpression(context dst.Expr, transaction dst.Expr) dst.Expr {
	return &dst.CallExpr{
		Fun: &dst.Ident{
			Name: "NewContext",
			Path: NewRelicAgentImportPath,
		},
		Args: []dst.Expr{
			dst.Clone(context).(dst.Expr),
			dst.Clone(transaction).(dst.Expr),
		},
	}
}

// WrapContextExpression creates an expression that injects a context with a transaction
// if async is true, the transaction will be cloned by calling NewGoroutine()
// this is protected from using the same object, and will always clone inputs
func WrapContextExpression(context dst.Expr, transaction string, async bool) dst.Expr {
	var txn dst.Expr
	txn = dst.NewIdent(transaction)
	if async {
		txn = TxnNewGoroutine(txn)
	}
	return &dst.CallExpr{
		Fun: &dst.Ident{
			Name: "NewContext",
			Path: NewRelicAgentImportPath,
		},
		Args: []dst.Expr{
			dst.Clone(context).(dst.Expr),
			txn,
		},
	}
}

// ContextParameter creates a field for a context parameter
func NewContextParameter(name string) *dst.Field {
	return &dst.Field{
		Names: []*dst.Ident{
			dst.NewIdent(name),
		},
		Type: &dst.Ident{
			Name: "Context",
			Path: "context",
		},
	}
}
