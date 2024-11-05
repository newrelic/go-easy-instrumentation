package codegen

import "github.com/dave/dst"

const DefaultContextParameter = "ctx"

// TransferTransactionToContext creates an expression that transfers a transaction from one context to another.
func TransferTransactionToContext(contextWithTransaction dst.Expr, contextWithoutTransaction dst.Expr) dst.Expr {
	return &dst.CallExpr{
		Fun: &dst.Ident{
			Name: "NewContext",
			Path: NewRelicAgentImportPath,
		},
		Args: []dst.Expr{
			dst.Clone(contextWithoutTransaction).(dst.Expr),
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "FromContext",
					Path: NewRelicAgentImportPath,
				},
				Args: []dst.Expr{
					dst.Clone(contextWithTransaction).(dst.Expr),
				},
			},
		},
	}
}

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

func ContextParameter(name string) *dst.Field {
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
