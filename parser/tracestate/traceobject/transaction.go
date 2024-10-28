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

type Transaction struct{}

func NewTransaction() *Transaction {
	return &Transaction{}
}

// AddToCall adds the transaction as an argument to the function call.
// If async is true, the transaction will cloned by calling NewGoroutine().
func (txn *Transaction) AddToCall(pkg *decorator.Package, call *dst.CallExpr, transactionVariable string, async bool) string {
	for _, arg := range call.Args {
		typ := util.TypeOf(arg, pkg)
		if typ != nil && typ.String() == transactionType {
			return ""
		}
	}

	// if we got this far, we did not find a suitable arguent in the function call
	// so we need to add it
	var txnArg dst.Expr
	txnArg = &dst.Ident{Name: transactionVariable}
	if async {
		txnArg = codegen.TxnNewGoroutine(txnArg)
	}
	call.Args = append(call.Args, txnArg)
	return codegen.NewRelicAgentImportPath
}

func needsTxnParameter(pkg *decorator.Package, params []*dst.Field) bool {
	for _, param := range params {
		typ := util.TypeOf(param.Type, pkg)
		if typ != nil && typ.String() == transactionType {
			return false
		}
	}

	return true
}

// AddToFuncDecl adds the transaction as a parameter to the function declaration
func (txn *Transaction) AddToFuncDecl(pkg *decorator.Package, decl *dst.FuncDecl) string {
	if needsTxnParameter(pkg, decl.Type.Params.List) {
		decl.Type.Params.List = append(decl.Type.Params.List, codegen.TxnAsParameter(codegen.DefaultTransactionVariable))
		return codegen.NewRelicAgentImportPath
	}
	return ""
}

// AddToFuncLit adds the transaction as a parameter to the function literal
func (txn *Transaction) AddToFuncLit(pkg *decorator.Package, lit *dst.FuncLit) string {
	if needsTxnParameter(pkg, lit.Type.Params.List) {
		lit.Type.Params.List = append(lit.Type.Params.List, codegen.TxnAsParameter(codegen.DefaultTransactionVariable))
		return codegen.NewRelicAgentImportPath
	}
	return ""
}

// This is already a transaction
func (txn *Transaction) AssignTransactionVariable(variableName string) (dst.Stmt, string) {
	// no-op
	return nil, ""
}
