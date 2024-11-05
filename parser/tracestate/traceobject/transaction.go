package traceobject

import (
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
)

const (
	transactionType = "*newrelic.Transaction"
)

type Transaction struct {
	transactionVariable string
}

func NewTransaction() *Transaction {
	return &Transaction{}
}

// AddToCall adds the transaction as an argument to the function call.
// If async is true, the transaction will cloned by calling NewGoroutine().
func (txn *Transaction) AddToCall(pkg *decorator.Package, call *dst.CallExpr, transactionVariable string, async bool) (TraceObject, string) {
	for i, arg := range call.Args {
		typ := util.TypeOf(arg, pkg)

		// check if this function contains a transaction argument
		ident, ok := arg.(*dst.Ident)
		if ok && ident.Name == transactionVariable {
			return NewTransaction(), codegen.NewRelicAgentImportPath
		}

		// if the call already contains a context, inject a transaction into it rather than adding an argument
		if typ != nil && typ.String() == contextType {
			call.Args[i] = codegen.WrapContextExpression(arg, transactionVariable, async)
			return NewContext(), codegen.NewRelicAgentImportPath
		}
	}

	// if we got this far, we did not find a suitable arguent in the function call
	// so we need to add it
	if async {
		call.Args = append(call.Args, codegen.TxnNewGoroutine(dst.NewIdent(transactionVariable)))
	} else {
		call.Args = append(call.Args, dst.NewIdent(transactionVariable))
	}

	return NewTransaction(), codegen.NewRelicAgentImportPath
}

func getTracingParameter(pkg *decorator.Package, params []*dst.Field) (TraceObject, string) {
	for _, param := range params {
		ident, ok := param.Type.(*dst.Ident)
		if ok && ident.Name == "Transaction" && ident.Path == codegen.NewRelicAgentImportPath {
			return NewTransaction(), codegen.NewRelicAgentImportPath
		}

		typ := util.TypeOf(param.Type, pkg)
		if typ != nil && typ.String() == contextType {
			return NewContext(), ""
		}
	}

	return nil, ""
}

// AddToFuncDecl adds the transaction as a parameter to the function declaration
func (txn *Transaction) AddToFuncDecl(pkg *decorator.Package, decl *dst.FuncDecl) (TraceObject, string) {
	obj, goGet := getTracingParameter(pkg, decl.Type.Params.List)
	if obj != nil {
		return obj, goGet
	}

	decl.Type.Params.List = append(decl.Type.Params.List, codegen.TxnAsParameter(codegen.DefaultTransactionVariable))
	return txn, codegen.NewRelicAgentImportPath
}

// AddToFuncLit adds the transaction as a parameter to the function literal
func (txn *Transaction) AddToFuncLit(pkg *decorator.Package, lit *dst.FuncLit) (TraceObject, string) {
	obj, goGet := getTracingParameter(pkg, lit.Type.Params.List)
	if obj != nil {
		return obj, goGet
	}

	lit.Type.Params.List = append(lit.Type.Params.List, codegen.TxnAsParameter(codegen.DefaultTransactionVariable))
	return txn, codegen.NewRelicAgentImportPath
}

// This is already a transaction
// no-op
func (txn *Transaction) AssignTransactionVariable(variableName string) (dst.Stmt, string) {
	return nil, ""
}
